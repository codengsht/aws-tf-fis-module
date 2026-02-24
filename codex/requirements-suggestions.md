# Requirements Review Suggestions

## Scope Alignment
- Keep `aws_fis_experiment_template` creation in scope.
- Treat this module as FIS enablement plus template orchestration, not IAM provisioning.
- Do not create IAM roles or IAM policies in this module.
- Do not create workload resources (Kinesis, DynamoDB, Lambda, network infrastructure).
- Support existing Lambda functions as targets only; remove Lambda module provisioning from requirements.

## Must-Fix Contradictions
- Resolve role reference behavior:
  - Replace ARN string construction with IAM role lookup by name (`FISExperimentRole`).
  - Require explicit failure if the role does not exist.
- Resolve missing target behavior conflict:
  - Current requirements contain both fail and skip semantics for missing target inputs.
  - Select one behavior and apply it consistently across requirements and outputs.
- Resolve CloudWatch log handling ambiguity:
  - Use `/aws/fis/{experiment_template_name}` as the log group naming pattern.
  - Clarify whether the module creates log groups or references caller-provided log groups.

## Security and Operability Gaps
- Tag-gating is not decided yet:
  - Keep enforcement deferred for this release.
  - Document deferred scope explicitly so it is not interpreted as missing implementation.
- Add explicit S3 bucket naming guardrails:
  - Sanitize `ci_commit_ref_name` to S3-compliant format (lowercase, allowed characters only).
  - Enforce bucket length and formatting constraints before apply-time failures.
- Clarify account boundary expectations:
  - Keep multi-account experiments out of scope.
  - Ensure target references are account-scoped by design.

## Suggested Requirement Rewrites
- Rewrite module dependency requirement:
  - Keep internal S3 and KMS module references.
  - Remove mandatory Lambda module reference from this module.
- Rewrite role requirement:
  - Default role name remains `FISExperimentRole`.
  - Resolve role via data lookup and output resolved ARN.
- Rewrite target/template validation requirement:
  - Define a strict input schema for templates, targets, actions, and stop conditions.
  - Validate `selection_mode` values (`ALL`, `COUNT(n)`, `PERCENT(n)`) with strict bounds.
- Rewrite outputs requirement:
  - Align output behavior with the chosen missing-target strategy (fail-fast or skip).
  - Include only outputs that can be deterministically produced under that strategy.

## Final Recommendation
- Proceed with a documentation-first refactor of `.kiro/specs/aws-fis-terraform-module/requirements.md` using the decisions below:
  - Templates remain in scope.
  - Role is resolved by IAM lookup using `FISExperimentRole`.
  - Tag enforcement is deferred.
  - Existing Lambda targets only; no Lambda resource provisioning.
- After requirements are updated, run a second pass to ensure acceptance criteria are non-conflicting and directly testable.



############# 4th after some changes Suggestions #############

Findings (ordered by severity)

High: Multi-account is declared out of scope, but not enforceable with current input rules.
No multi-account is stated in requirements.md, yet target inputs still allow arbitrary ARNs/identifiers with only non-empty validation in requirements.md and requirements.md.
Impact: Cross-account targets can slip through until runtime.

High: S3 naming still uses raw ci_commit_ref_name without sanitization requirements.
You require ci_commit_ref_name in requirements.md and directly interpolate it in requirements.md.
Impact: Common branch/tag characters (/, _, uppercase) can break bucket naming and fail applies unexpectedly.

Medium: Hardcoded FISExperimentRole may block multi-env reuse.
Role name is fixed in requirements.md and requirements.md.
Impact: If environments use different role names, module reuse requires code changes instead of input configuration.

Medium: Defaulting stop condition to source = "none" is operationally risky.
Current behavior in requirements.md.
Impact: Experiments can run without alarm guardrails unless callers remember to add them.

Medium: Terratest requirements cover happy path only; key failure modes are untested.
Current tests in requirements.md focus on create/outputs.
Impact: Regressions in validation behavior (invalid selection_mode, missing role, invalid bucket name, cross-account input) may not be caught early.

Low: Design artifact is still empty, which leaves implementation details implicit.
See design.md.
Impact: Higher chance of divergent implementation decisions despite good requirements.

Open questions

Do you want strict rejection of cross-account ARNs at input time, or only documentation-level “out of scope”?
Should ci_commit_ref_name be replaced by CI_COMMIT_REF_SLUG semantics (or explicit sanitization rules)?
Do you want to keep hardcoded role name, or allow override with default FISExperimentRole?
Should stop_conditions = none require explicit opt-in flag?