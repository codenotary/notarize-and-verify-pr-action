# GitHub action for notarizing and verifying pull requests using VCN

Uses VCN to notarize the latest commit in the PR for each approver and then verify it was notarized by each signer ID in a configured list of signer IDs.

## Build Docker image and publish it to DockerHub

`docker build -t <your-docker-hub-username>/notarize-and-verify-commit .`

`docker push <your-docker-hub-username>/notarize-and-verify-commit`

## Usage

See ./.github/workflows/test.yml
