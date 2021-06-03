# GitHub action for notarizing and verifying pull requests using VCN

This official action from CodeNotary allows to notarize the latest commit on a Pull Request for each approver, and then verify it was notarized by a configurable list of signer IDs.

The action can be referenced as a [Docker image from your workflow](https://docs.github.com/en/actions/learn-github-actions/finding-and-customizing-actions#referencing-a-container-on-docker-hub).

## Build your own image

If you want to produce an artifact for the action from the code, you can build the action yourself and publish it to your own registry:

`docker build -t <your-docker-hub-username>/notarize-and-verify-commit .`

`docker push <your-docker-hub-username>/notarize-and-verify-commit`

## Usage

See [./.github/workflows/test.yml](.github/workflows/test.yml) for a full example.

In summary, create a step using the action and give as parameters:

- Immutable Ledger REST API URL
- Immutable Ledger REST API personal token
- Immutable Ledger gRPC end-point host
- Immutable Ledger gRPC end-point port
- Immutable Ledger Ledger ID you will use to authenticate
- A comma separated list of Signer IDs that need to approve the Pull Request to pass
- Keep ${{ github.event.review.user.login }} so that the PR is notarized by the current user on approval

```yaml
args: >
  http://<CNIL-REST-API-host>:8000/api/v1
  Dal8SLBVMFq3363s5h4AgFrN6U5tjoNC4vmSoPoPIZlWy1M2-nci-MuRLoueCNlh
  <CNIL-gRPC-API-host>
  3324
  42139843608656000
  padurean,padurean-alt
  ${{ github.event.review.user.login }}
```

Once a PR is ready for review, each approval will create a notarization. The action will succeed once all listed Signer IDs have notarized.
