## What Changed

Describe the change in concrete terms. Keep this focused on the behavior, files, or workflow that changed.

## Why

Explain why this change is needed. Link the relevant issue, task, requirement, or bug when one exists.

## Testing

List the validation you performed and the result for each item.

```bash
make test
make lint
make build
```

If you did not run one of the standard checks, say why. If automated coverage was not practical, describe the manual verification you performed instead.

## Follow-Up Work / Limitations

Call out any known limitations, deferred cleanup, or related work that should happen in a separate PR. If none, say `None`.

## Reviewer Notes

Include screenshots, terminal output, or extra context only when they materially help review.

## Checklist

- [ ] This PR is focused on one problem and does not include unrelated cleanup
- [ ] Tests were added or updated where appropriate
- [ ] `make test` was run locally, or the reason it was not run is documented above
- [ ] `make lint` was run locally, or the reason it was not run is documented above
- [ ] `make build` was run if the change affects compilation, packaging, CLI wiring, or dependencies
- [ ] `go mod tidy` was run if imports or dependencies changed
- [ ] Documentation was updated where appropriate
- [ ] Any testing gaps, manual verification, follow-up work, or known limitations are documented above
