# CLAUDE.md

## Purpose

This project provides a Go program plus a Claude plugin layer that configures Claude Code integration artifacts such as hooks, templates, and global include wiring.

The repository must always be treated as a two-phase system:

1. Go binary lifecycle
2. Claude plugin lifecycle

Any change that affects installation, upgrade, versioning, packaging, setup, or documentation must evaluate both phases explicitly.

---

## Non-negotiable product rules

### 1. Always treat installation and upgrade as two phases

Never describe installation as a single opaque action if it actually consists of:

- installing or updating the Go binary
- installing or updating the Claude plugin that configures hooks, templates, and global Claude integration

Any implementation, fix, release, or documentation change must preserve this distinction.

If global Claude integration depends on injecting or maintaining an include in the user's global `CLAUDE.md`, that requirement must be treated as part of plugin installation, not as an optional footnote.

Do not collapse binary installation and plugin installation into one conceptual step.
Do not imply the plugin works correctly without the Go program being present.
Do not imply the Go program alone completes Claude integration.

### 2. Version consistency is mandatory

When creating or updating a version, all version references that are expected to expose or describe the current version must be updated together.

Never update only one visible version string and leave others behind.
Never assume a tag alone is enough.
Never assume `main.go` alone is enough.
Never assume `plugin.json` alone is enough.
Never assume README examples can lag behind.

The repository must not ship inconsistent visible versions across code, plugin metadata, installation examples, templates, scripts, or release artifacts.

If a new version is created, explicitly verify all files that reference the version and update them before considering the work complete.

Examples of places that commonly require review:

- Go source version constant or variable
- plugin manifest files such as `plugin.json`
- README installation or upgrade examples
- docs that print example version output
- release workflow metadata
- scripts that embed a version
- templates rendered into user environments
- package manager metadata
- Git tags and release titles

This list is illustrative, not exhaustive.
The rule is consistency across every version-bearing surface, not consistency across only a preferred subset.

### 3. Do not oversell the product

Do not use inflated claims, vague marketing language, or statements that suggest capabilities the code does not actually implement.

Avoid terms such as:

- intelligent compression
- semantic optimization
- smart deduplication
- zero-cost abstraction
- zero dependencies

unless those claims are demonstrably true in the implementation and remain true across supported installation paths.

If the system reduces output through line filtering, truncation, grouping, exclusion rules, comment stripping, or similar techniques, describe it honestly as heuristic pruning, heuristic filtering, or heuristic output reduction.

Prefer precise wording over attractive wording.

Good:
- heuristic pruning of noisy tool output
- rule-based reduction of shell output before it reaches Claude
- deterministic filtering and truncation to reduce context waste

Bad:
- intelligent context optimization
- semantic compression engine
- smart deduplication pipeline

### 4. Never claim features that are only partially implemented

If a feature exists only in one module, one code path, one command family, or one experimental workflow, do not present it as a general property of the whole system.

Examples:
- If truncation exists but true deduplication is not implemented globally, do not claim global deduplication.
- If gain tracking exists separately but is not integrated as a guaranteed workflow behavior, do not describe it as an always-on capability.
- If a setup step is conditional or platform-specific, document that constraint explicitly.

### 5. Documentation must describe reality, not intention

README, plugin docs, templates, help text, release notes, and setup guides must reflect what the code does now.

Do not leave “aspirational” wording in user-facing docs.
Do not document future behavior in present tense.
Do not keep stale examples after changing actual behavior.

If implementation and docs disagree, fix the disagreement, not just the symptom.

---

## Installation and setup rules

### 6. Binary and plugin responsibilities must stay separated

The Go binary is responsible for executable behavior.
The plugin is responsible for Claude integration behavior such as hooks, templates, and global include setup.

When changing install flows, do not blur these responsibilities.

Document them separately.
Test them separately.
Version them consistently.
Reason about failure modes separately.

### 7. Global Claude include handling is part of the plugin contract

If the plugin injects or maintains an include inside the user's global `CLAUDE.md`, that behavior is core setup logic.

Changes touching include injection must verify:

- include creation works when absent
- include update works when already present
- duplicate include insertion does not occur
- path resolution rules remain correct
- uninstall or reinstall does not leave broken state
- behavior is correct whether the project lives inside or outside the user's home path rules

### 8. Path-derived naming must follow project rules exactly

If project naming is derived from filesystem location, always apply the current canonical repository rule:

- when inside home, derive from the full path from home
- when outside home, derive from the current working directory base path rule defined by the project

Do not simplify naming logic in docs or code examples.
Do not hardcode examples that contradict the rule.
Do not silently replace canonical naming with a prettier naming scheme.

If touching name generation logic, update docs and examples accordingly.

---

## Versioning rules

### 9. A version bump is not complete until all version surfaces are checked

Any task that creates, bumps, or references a version must include an explicit repository-wide review of version-bearing files.

Minimum expectation:
- search the repository for the previous version
- search the repository for version display patterns
- inspect manifests, docs, scripts, templates, and source constants
- verify the release/tag matches what the repository exposes

Do not stop after updating the first obvious file.

### 10. Examples must not leak old versions

Examples in documentation are part of the user-facing version surface.

If the README says one version, the plugin manifest says another, and the binary prints another, that is a release defect.

Treat stale examples as correctness issues, not cosmetic issues.

---

## Testing and validation rules

### 11. Functional validation is required before commit when behavior changes

For any change affecting setup, installation, hooks, filters, templates, versioning, path handling, manifests, or generated user files, perform functional validation before commit.

Do not rely only on compilation.
Do not rely only on unit tests if the change affects integration behavior.
Do not assume a patch or minor release is low risk.

At minimum, validate the behavior actually changed by the work.

Examples of functional validation:
- install binary, then install plugin, and confirm both phases work
- verify hook registration really occurs
- verify include injection really occurs
- verify duplicate include injection does not occur
- verify an upgrade path updates existing state correctly
- verify a version bump is reflected everywhere expected
- verify generated files match the current templates
- verify command output still matches documented examples when relevant

### 12. Patch and minor releases must still be treated as potentially breaking

Small version increments often fix one bug and introduce another.

Therefore:
- do not assume patch means safe
- do not assume minor means documentation-only impact
- do not skip functional tests because the diff looks small

Any release that changes install flow, generated files, matching logic, filtering, version references, or plugin behavior must be functionally tested.

### 13. Test the real user path, not only internal helpers

If the user experience depends on commands executed in sequence, test the sequence, not only isolated functions.

Typical real path:
1. install or update Go binary
2. install or update Claude plugin
3. verify generated Claude integration files
4. verify global `CLAUDE.md` include state
5. verify hooks actually execute
6. verify output pruning behavior is still correct

A helper passing in isolation is not enough if the end-to-end path fails.

---

## Editing rules for agents

### 14. Prefer repository-wide consistency over local fixes

When editing one file, always consider whether the same concept appears elsewhere.

Typical cross-file consistency areas:
- version strings
- installation wording
- plugin terminology
- setup flow descriptions
- claims about filtering behavior
- generated template content
- command examples
- path naming logic

Do not make a local wording fix that leaves contradictory wording elsewhere.

### 15. Be explicit about what is heuristic

Whenever describing output reduction behavior, prefer precise, bounded language.

Use:
- heuristic pruning
- rule-based filtering
- deterministic truncation
- comment stripping
- output grouping
- noise reduction

Avoid:
- semantic understanding
- intelligent summarization
- deduplication

unless the implementation truly supports those claims in the relevant scope.

### 16. Do not invent abstractions the project does not need

Do not introduce broader architectural language unless it solves a real repository problem.

Keep terminology simple:
- binary
- plugin
- hook
- template
- install
- upgrade
- version
- heuristic pruning

This project benefits from clarity, not conceptual inflation.

---

## Commit and release hygiene

### 17. Before finalizing a change, verify these questions

- Does this preserve the two-phase install model?
- Does plugin setup still depend on and correctly integrate with the Go binary?
- Are global include behaviors still correct?
- Are all version references consistent?
- Does documentation describe current behavior exactly?
- Are any claims overstated?
- Was the changed behavior functionally tested?
- Are examples still valid?

If any answer is “no” or “unknown”, the work is not complete.

### 18. Never close a versioning task on partial repository updates

A task about “bump version”, “prepare release”, “update plugin”, “refresh docs”, or similar is incomplete if any visible version reference still points to an older version.

### 19. Never close a setup task on a simulated path only

If a task changes installation or upgrade behavior, completion requires validating the actual user path, not just code compilation or manifest editing.

---

## Examples

### Example: correct installation wording

Good:
1. Install or update the Go binary.
2. Install or update the Claude plugin so hooks, templates, and global `CLAUDE.md` include wiring are configured.

Bad:
Install the plugin and everything is ready.

Bad:
Run the Go installer to fully configure Claude integration.

### Example: correct product wording

Good:
This tool applies heuristic pruning to noisy tool output before it reaches Claude Code.

Good:
The hook uses deterministic filtering and truncation rules to reduce wasted context.

Bad:
This tool performs intelligent semantic compression of shell output.

Bad:
This tool deduplicates and optimizes all output automatically.

### Example: correct versioning behavior

Good:
Update the Go version constant, plugin manifest, README examples, template output, and release tag together.

Bad:
Create tag `vX.Y.Z` and update only `main.go`.

### Example: correct release validation

Good:
After bumping the version, search the repository for the old version string and verify no stale references remain in source, docs, manifests, scripts, or templates.

Bad:
Assume CI or the tag name proves the repository is consistent.

---

## Default working stance

When modifying this repository:

- assume installation is two-phase
- assume version consistency must be enforced globally
- assume docs must be literal and precise
- assume marketing wording is harmful unless proven true
- assume small releases can still break behavior
- assume functional validation is required for user-facing changes

If there is tension between attractive wording and accurate wording, choose accurate wording.
If there is tension between a quick local fix and repository-wide consistency, choose consistency.
If there is tension between “probably works” and “validated”, choose validation.