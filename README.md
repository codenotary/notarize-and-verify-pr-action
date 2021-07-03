# GitHub action for notarizing and verifying pull requests using VCN

This official GitHub action from CodeNotary allows to notarize the latest commit on a Pull Request for each approver, and then verify it was notarized by a configurable list of signer IDs.

## Usage

See [./.github/workflows/test.yml](.github/workflows/test.yml) for a full example.

In summary, checkout the repo at the latest PR commit and then create a step using this action and pass it the following parameters:

- Immutable Ledger REST API URL
- Immutable Ledger REST API personal token
- Immutable Ledger gRPC end-point host
- Immutable Ledger gRPC end-point port
- Immutable Ledger Ledger ID you will use to authenticate
- A comma separated list of Signer IDs that need to approve the Pull Request to pass
- Keep ${{ github.event.review.user.login }} so that the PR is notarized by the current user on approval

```yaml
    steps:
      - name: Checkout repository
        uses: actions/checkout@v2
        with:
          ref: ${{ github.event.review.commit_id }}

      - name: Notarize and verify PR
        uses: codenotary/notarize-and-verify-pr-action@v1.0.0
        with:
          cnil_rest_api_url: http://<CNIL-REST-API-host>:8000/api/v1
          cnil_personal_token: Dal8SLBVMFq3363s5h4AgFrN6U5tjoNC4vmSoPoPIZlWy1M2-nci-MuRLoueCNlh
          cnil_grpc_host: <CNIL-gRPC-API-host>
          cnil_grpc_port: 3324
          cnil_ledger: 42139843608656000
          required_pr_approvers: dmacvicar,padurean-alt
          current_pr_approver: ${{ github.event.review.user.login }}
```

Once a PR is ready for review, each approval will create a notarization. The action will succeed once all listed Signer IDs have notarized.

## Build and publish the Docker image

If you want to produce an artifact for the action from the code, you can build the action yourself and publish it to your own registry:

`docker build -t <docker-hub-username>/notarize-and-verify-pr .`

`docker push <docker-hub-username>/notarize-and-verify-pr`