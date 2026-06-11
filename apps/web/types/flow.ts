export interface FlowWorkflow {
  id: string
  name: string
  version: string
  description: string
  path: string
}

export interface FlowWorkflowVariable {
  key: string
  type: string
  required: boolean
  description: string
}

export interface FlowWorkflowStep {
  id: string
  kind: string
  agent?: string
  approverPolicy?: string
}

export interface FlowWorkflowDetail extends FlowWorkflow {
  variables: FlowWorkflowVariable[]
  steps: FlowWorkflowStep[]
}

export interface FlowTimelineEvent {
  at: string
  type: string
  detail: string
}

export interface FlowStepRecord {
  runId: string
  stepId: string
  status: string
  workerId: string
  startedAt: string
  finishedAt?: string
  kind?: string
  agent?: string
}

export interface FlowHandoffRecord {
  runId: string
  stepId: string
  status: string
  kind?: string
  agent?: string
  workerId: string
  sessionId?: string
  dispatchAttemptAt?: string
  createdAt: string
  updatedAt: string
}

export interface FlowApprovalRecord {
  id: string
  runId: string
  stepId: string
  kind: string
  status: string
  approverPolicy?: string
  requestedBy: string
  decidedBy?: string
  rationale?: string
  evidenceManifest?: string
  createdAt: string
  decidedAt?: string
}

export interface FlowRun {
  id: string
  workflowId: string
  status: string
  currentStep: string
  branch: string
  worktreePath: string
  createdAt?: string
  timeline: FlowTimelineEvent[]
  steps: FlowStepRecord[]
  handoffs: FlowHandoffRecord[]
}

export interface FlowAdvanceRunResult extends FlowRun {}
export interface FlowStepDispatchResult extends FlowRun {}
export interface FlowStepCompletionResult extends FlowRun {}
export interface FlowStepFailureResult extends FlowRun {}

export interface FlowValidationIssue {
  field: string
  code: string
  message: string
}

export interface FlowValidationResult {
  workflowId: string
  valid: boolean
  errors: FlowValidationIssue[]
  warnings: FlowValidationIssue[]
}

export interface FlowDryRunStep {
  id: string
  kind: string
  agent?: string
  blocking: boolean
}

export interface FlowDryRunResult {
  workflowId: string
  ready: boolean
  missingVariables: string[]
  warnings: FlowValidationIssue[]
  steps: FlowDryRunStep[]
}

export interface ListResponse<T> {
  items: T[]
}
