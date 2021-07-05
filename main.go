package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	vcnAPI "github.com/vchain-us/vcn/pkg/api"
	vcnGitExtractor "github.com/vchain-us/vcn/pkg/extractor/git"
	vcnMeta "github.com/vchain-us/vcn/pkg/meta"
	vcnStore "github.com/vchain-us/vcn/pkg/store"
	vcnURI "github.com/vchain-us/vcn/pkg/uri"
)

const (
	pathToRepo     = "/github/workspace"
	identitySuffix = "@github"
	httpTimeout    = 30 * time.Second
)

const (
	red    = "\033[1;31m%s\033[0m"
	green  = "\033[1;32m%s\033[0m"
	yellow = "\033[1;33m%s\033[0m"
)

var (
	errAPIKeyNotFound = errors.New("API key not found")
)

// Expects args:
//	- CNIL REST API URL
//	- CNIL REST API personal token
//	- CNIL gRPC host
//	- CNIL gRPC port
//  - CNIL gRPC no TLS
//	- CNIL ledger ID
//	- comma-separated list of required PR approvers (GitHub usernames)
//	- GitHub username (signer ID) of the current PR approver
func main() {

	// validate number of inputs
	if len(os.Args)-1 != 8 {
		fmt.Printf(red, fmt.Sprintf(
			"invalid args %+v: expected 7, got %d\n", os.Args, len(os.Args)-1))
		os.Exit(1)
	}

	// validate inputs
	cnilURL := strings.TrimSuffix(requireArg(1, "CNIL REST API URL"), "/")
	cnilToken := requireArg(2, "CNIL REST API personal token")
	cnilHost := requireArg(3, "CNIL gRPC API host")
	cnilPort := requireArg(4, "CNIL gRPC API port")
	cnilNoTLS := requireArg(5, "CNIL gRPC no TLS")
	cnilLedgerID := requireArg(6, "CNIL ledger ID")
	requiredApprovers := requireArg(7, "required PR approvers")
	approver := requireArg(8, "PR approver")

	var err error
	var noTLS bool
	if len(cnilNoTLS) > 0 {
		noTLS, err = strconv.ParseBool(cnilNoTLS)
		if err != nil {
			fmt.Print(red, fmt.Sprintf(
				"ABORTING: error parsing the \"no TLS\" argument value \"%s\": %v\n",
				cnilNoTLS, err))
			os.Exit(1)
		}
	}

	// get and rotate or create API keys for each required approver
	cnilAPIOptions := &cnilOptions{baseURL: cnilURL, token: cnilToken, ledgerID: cnilLedgerID}
	apiKeyPerRequiredApprover := make(map[string]string)
	if err := getAndRotateOrCreateAPIKeys(
		cnilAPIOptions,
		requiredApprovers,
		&apiKeyPerRequiredApprover,
	); err != nil {
		fmt.Printf(red, fmt.Sprintf("ABORTING: %v\n", err))
		os.Exit(1)
	}

	// create VCN artifact from the git repository folder
	artifact, err := vcnArtifactFromGitRepo()
	if err != nil {
		fmt.Printf(red, fmt.Sprintf(
			"ABORTING: error creating VCN artifact from git repo %s: %v\n", pathToRepo, err))
		os.Exit(1)
	}

	// make sure the local VCN store directory exists
	options := &vcnOptions{
		storeDir: "./.vcn",
		cnilHost: cnilHost,
		cnilPort: cnilPort,
		noTLS:    noTLS,
	}
	if err := os.MkdirAll(options.storeDir, os.ModePerm); err != nil {
		fmt.Printf(red, fmt.Sprintf(
			"error creating VCN local store directory %s: %v\n", options.storeDir, err))
	}
	// initialize VCN store
	vcnStore.SetDir(options.storeDir)
	vcnStore.LoadConfig()

	// notarize the git repository artifact for the current PR approver (if required)
	if notarizationKey, ok := apiKeyPerRequiredApprover[approver]; ok {
		fmt.Println("\nNotarizing PR ...")
		options.cnilAPIKey = notarizationKey
		if err := notarize(artifact, options); err != nil {
			fmt.Printf(red, fmt.Sprintf("ABORTING: notarization error: %v\n", err))
			os.Exit(1)
		}
		fmt.Printf(green, fmt.Sprintf(
			"Successfully notarized PR for current approver %s\n", approver))
	} else {
		fmt.Printf(green, fmt.Sprintf(
			"SKIPPING notarization: PR approver %s is not required\n", approver))
	}

	// verify if the git repository was notarized for every required PR approver
	var notarizedApprovers []string
	fmt.Printf(
		"\nVerifying if the PR has been notarized for all %d required PR approvers ...\n",
		len(apiKeyPerRequiredApprover))
	for requiredApprover, apiKey := range apiKeyPerRequiredApprover {

		fmt.Printf(
			"\n   Verifying if the PR has been notarized for %s ...\n",
			requiredApprover)

		options.cnilAPIKey = apiKey
		cnilArtifact, err := verify(artifact, options)
		if err != nil {
			fmt.Printf(red, fmt.Sprintf(
				"   ABORTING: error verifying PR for required approver %s: %v\n",
				requiredApprover, err))
			os.Exit(1)
		}
		if cnilArtifact == nil {
			fmt.Printf(yellow, fmt.Sprintf(
				"   PR is NOT notarized for required approver %s\n", requiredApprover))
			continue
		}

		if cnilArtifact.Status == vcnMeta.StatusTrusted {
			notarizedApprovers = append(notarizedApprovers, requiredApprover)
		}

		cnilArtifactDetails := fmt.Sprintf(`
      Status:     %s
      PR commit:  %s
      Signer ID:  %s
`,
			coloredStatus(cnilArtifact.Status),
			cnilArtifact.Name,
			cnilArtifact.Signer)

		fmt.Printf(
			"   Verification details for approver %s: %s", requiredApprover, cnilArtifactDetails)

	}
	fmt.Println("")

	// DO NOT succeed if the git repository IS NOT notarized for all required PR approvers
	if len(notarizedApprovers) != len(apiKeyPerRequiredApprover) {
		fmt.Printf(yellow, fmt.Sprintf(
			"PR is notarized for %d of %d required approvers:\n"+
				"   - notarized: %s\n   - required : %s",
			len(notarizedApprovers), len(apiKeyPerRequiredApprover),
			strings.Join(notarizedApprovers, ","), requiredApprovers))
		os.Exit(1)
	}

	// DO succeed if the git repository IS notarized for all required PR approvers
	fmt.Printf(green, fmt.Sprintf(
		"PR is notarized for all %d required approvers (%s).",
		len(apiKeyPerRequiredApprover), requiredApprovers))
}

func requireArg(argIndex int, argName string) string {
	argVal := strings.TrimSpace(os.Args[argIndex])
	// fmt.Printf("  - %s: %s (length: %d)\n", argName, argVal, len(argVal))
	if len(argVal) == 0 {
		fmt.Printf(red, fmt.Sprintf("ABORTING: required argument value %s is empty\n", argName))
		os.Exit(1)
	}
	return argVal
}

type cnilOptions struct {
	baseURL  string
	token    string
	ledgerID string
}

func getAndRotateOrCreateAPIKeys(
	options *cnilOptions,
	requiredApprovers string,
	apiKeyPerRequiredApprover *map[string]string,
) error {
	for i, requiredApprover := range strings.Split(requiredApprovers, ",") {
		requiredApprover = strings.TrimSpace(requiredApprover)
		if len(requiredApprover) == 0 {
			fmt.Printf(yellow, fmt.Sprintf(
				"SKIPPING empty approver on position %d in the list of required approvers\n", i))
			continue
		}
		signerID := requiredApprover + identitySuffix
		apiKey, err := getAPIKey(options, signerID)
		if errors.Is(err, errAPIKeyNotFound) {
			apiKey, err = createAPIKey(options, signerID)
		} else if err == nil {
			apiKey, err = rotateAPIKey(options, apiKey.ID)
		}
		if err != nil {
			return fmt.Errorf("error getting or creating / rotating API key for approver %s: %v",
				requiredApprover, err)
		}
		(*apiKeyPerRequiredApprover)[requiredApprover] = apiKey.Key
	}
	return nil
}

type APIKeyResponse struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type APIKeysPageResponse struct {
	Total uint64            `json:"total"`
	Items []*APIKeyResponse `json:"items"`
}

func getAPIKey(options *cnilOptions, signerID string) (*APIKeyResponse, error) {
	url := fmt.Sprintf(
		"%s/api_keys/identity/%s", options.baseURL, url.PathEscape(signerID))
	responsePayload := APIKeysPageResponse{}
	if err := sendHTTPRequest(
		http.MethodGet,
		url,
		options.token,
		http.StatusOK,
		nil,
		&responsePayload,
	); err != nil {
		return nil, err
	}

	if len(responsePayload.Items) == 0 {
		return nil, errAPIKeyNotFound
	}

	return responsePayload.Items[0], nil
}

type APIKeyCreateReq struct {
	Name     string `json:"name"`
	ReadOnly bool   `json:"read_only"`
}

func createAPIKey(options *cnilOptions, signerID string) (*APIKeyResponse, error) {
	url := fmt.Sprintf("%s/ledgers/%s/api_keys", options.baseURL, options.ledgerID)
	payload := APIKeyCreateReq{Name: signerID}
	payloadJSON, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf(
			"error JSON-marshaling POST %s request with payload %+v: %v",
			url, payload, err)
	}
	responsePayload := APIKeyResponse{}
	if err := sendHTTPRequest(
		http.MethodPost,
		url,
		options.token,
		http.StatusCreated,
		bytes.NewBuffer(payloadJSON),
		&responsePayload,
	); err != nil {
		return nil, err
	}

	return &responsePayload, nil
}

func rotateAPIKey(options *cnilOptions, apiKeyID string) (*APIKeyResponse, error) {
	url := fmt.Sprintf("%s/ledgers/%s/api_keys/%s/rotate", options.baseURL, options.ledgerID, apiKeyID)
	responsePayload := APIKeyResponse{}
	if err := sendHTTPRequest(
		http.MethodPut,
		url,
		options.token,
		http.StatusOK,
		nil,
		&responsePayload,
	); err != nil {
		return nil, err
	}

	return &responsePayload, nil
}

func sendHTTPRequest(
	method string,
	url string,
	token string,
	expectedStatus int,
	payload io.Reader,
	responsePayload interface{},
) error {
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return fmt.Errorf("error creating HTTP request %s %s: %v", method, url, err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+token)

	response, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return fmt.Errorf("error sending request %s %s: %v", method, url, err)
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("%s %s: error reading response body: %v", method, url, err)
	}

	if response.StatusCode != expectedStatus {
		return fmt.Errorf("%s %s error: expected response status %d, got %s with body %s",
			method, url, expectedStatus, response.Status, responseBody)
	}

	if err := json.Unmarshal(responseBody, responsePayload); err != nil {
		return fmt.Errorf("error JSON-unmarshaling %s %s response body %s: %v",
			method, url, responseBody, err)
	}

	return nil
}

type vcnOptions struct {
	storeDir   string
	cnilHost   string
	cnilPort   string
	cnilAPIKey string
	noTLS      bool
}

func vcnArtifactFromGitRepo() (*vcnAPI.Artifact, error) {
	repoURI, err := vcnURI.Parse("git://" + pathToRepo)
	if err != nil {
		return nil, fmt.Errorf("error parsing path to repo: %v", err)
	}

	vcnArtifact, err := vcnGitExtractor.Artifact(repoURI)
	if err != nil {
		return nil, fmt.Errorf("error creating artifact: %v", err)
	}

	return vcnArtifact[0], nil
}

func notarize(vcnArtifact *vcnAPI.Artifact, options *vcnOptions) error {
	vcnCNILUser, err := vcnAPI.NewLcUser(
		options.cnilAPIKey, "", options.cnilHost, options.cnilPort, "", false, options.noTLS)
	if err != nil {
		return fmt.Errorf("error initializing vcn client: %v", err)
	}
	if err := vcnCNILUser.Client.Connect(); err != nil {
		return fmt.Errorf("error connecting vcn client: %v", err)
	}
	defer vcnCNILUser.Client.Disconnect()

	var state vcnMeta.Status
	_, _, err = vcnCNILUser.Sign(*vcnArtifact, vcnAPI.LcSignWithStatus(state))
	if err != nil {
		return fmt.Errorf("error signing artifact: %v", err)
	}

	return nil
}

func verify(artifact *vcnAPI.Artifact, options *vcnOptions) (*vcnAPI.LcArtifact, error) {
	vcnCNILUser, err := vcnAPI.NewLcUser(
		options.cnilAPIKey, "", options.cnilHost, options.cnilPort, "", false, options.noTLS)
	if err != nil {
		return nil, fmt.Errorf("error initializing vcn client: %v", err)
	}
	if err := vcnCNILUser.Client.Connect(); err != nil {
		return nil, fmt.Errorf("vcn connection error: %v", err)
	}
	defer vcnCNILUser.Client.Disconnect()

	cnilArtifact, verified, err := vcnCNILUser.LoadArtifact(artifact.Hash, "", "", 0)
	if err == vcnAPI.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger might be compromised: %v", err)
	}

	if !verified {
		return nil, errors.New(
			`ledger might be compromised: CNIL verification status is "false"`)
	}

	if cnilArtifact.Revoked != nil && !cnilArtifact.Revoked.IsZero() {
		cnilArtifact.Status = vcnMeta.StatusApikeyRevoked
	}

	return cnilArtifact, nil
}

func coloredStatus(status vcnMeta.Status) string {
	statusColor := green
	switch status {
	case vcnMeta.StatusUntrusted, vcnMeta.StatusUnknown, vcnMeta.StatusUnsupported:
		statusColor = red
	case vcnMeta.StatusApikeyRevoked:
		statusColor = yellow
	}
	return fmt.Sprintf(statusColor, status)
}
