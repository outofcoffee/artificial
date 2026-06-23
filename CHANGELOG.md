# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [Unreleased]
### Added
- feat: add `Outfit` files — declarative, Dockerfile-style provider selections
  applied with `outfit apply` (defaults to `./Outfit`). Supports `PROVIDER`,
  `FAMILY`, `MODEL`, `CONTEXT`, and `BASEURL` instructions
- feat: add `outfit export` to capture the current config as an `Outfit`

### Changed
- **BREAKING:** rename the CLI and binary from `oc-config` to `outfit`. Reinstall
  from the Homebrew tap (`brew install lucinate-ai/tap/outfit`) or rebuild from
  source, and rename the `OC_CONFIG_PROVIDERS`/`OC_CONFIG_BASE_URL` environment
  variables to `OUTFIT_PROVIDERS`/`OUTFIT_BASE_URL`
- **BREAKING:** move the repository and Go module to
  `github.com/lucinate-ai/outfit` (was `github.com/outofcoffee/configure-opencode`)
- docs: document the `Outfit` file format in `docs/outfit-file.md`
- docs: move the llama.cpp guides under `examples/`, each with an `Outfit`

## [0.2.0] - 2026-06-22
### Added
- feat: allow the provider catalogue to be overridden at runtime

### Changed
- build: add Makefile with build, test, and coverage targets
- ci: publish binary to Homebrew tap on release
- docs: add llama.cpp Qwen3.6 guide and link from README
- docs: add llama.cpp guide for Gemma-4 with MTP
- docs: adds changelog

## [0.1.0] - 2026-06-21
### Added
- feat: add opencode OpenRouter DeepSeek V4 config script
- feat: generalise outfit into a multi-provider opencode configurator

### Changed
- ci: add test/build and tag-driven release workflows
- docs: add README and AGENTS.md
- refactor: rewrite opencode config tool in Go with JSONC merge

### Other
- Initial commit
