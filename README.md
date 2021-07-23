# GitHub action for notarizing and verifying pull requests using VCN

This official GitHub action from CodeNotary allows to notarize the latest commit on a Pull Request for each approver, and then verify it was notarized by a configurable list of signer IDs.

## Usage

:bulb: See [./.github/workflows/test.yml](.github/workflows/test.yml) for a full example and the complete list of supported inputs.

In summary, checkout the repo at the latest PR commit and then create a step using this action and pass it the following parameters:

- Immutable Ledger host
- Keep ${{ github.event.review.user.login }} so that the PR is notarized by the current user on approval
- Comma separated list of API keys which will be used during notarization and verification.
These also determine the required approvers: they will be the signer IDs (i.e. names) of the API keys.
  - :warning: the signer IDs must match GitHub usernames - either as `<github-username>@github` either simply `<github-username>`. Example value: `ghuser1@github.xxx...,ghuser2.yyy...`
  - :warning: if this input is specified, the following inputs will be ignored

```yaml
    steps:
      - name: Checkout repository
        uses: actions/checkout@v2
        with:
          ref: ${{ github.event.review.commit_id }}

      - name: Notarize and verify PR
        uses: codenotary/notarize-and-verify-pr-action@v2.0.0
        with:
          cnil_host: your-immutable-ledger-host
          current_pr_approver: ${{ github.event.review.user.login }}
          cnil_api_keys: ghuser1@github.UnCbnnjZjruUTVsconoGicRmYeVcwxdhwPke,ghuser2@github.GGkYcktPZGYJSFtMUvuKvmeosDGKPIeSVJIB
```

Once a PR is ready for review, each approval will create a notarization. The action will succeed once all listed Signer IDs have notarized.

## How to build and publish the Docker image

If you want to produce an artifact for the action from the code, you can build the action yourself and publish it to your own registry:

`docker build -t <docker-hub-username>/notarize-and-verify-pr .`

`docker push <docker-hub-username>/notarize-and-verify-pr`
