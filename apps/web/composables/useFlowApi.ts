import type { FlowAdvanceRunResult, FlowDryRunResult, FlowRun, FlowStepCompletionResult, FlowStepDispatchResult, FlowStepFailureResult, FlowValidationResult, FlowWorkflow, FlowWorkflowDetail, ListResponse } from '~/types/flow'

export const useFlowApi = () => {
  const config = useRuntimeConfig()
  const baseURL = config.public.apiBase

  const getWorkflows = async () => {
    const data = await $fetch<ListResponse<FlowWorkflow>>('/workflows', { baseURL })
    return data.items ?? []
  }

  const getRuns = async () => {
    const data = await $fetch<ListResponse<FlowRun>>('/runs', { baseURL })
    return data.items ?? []
  }

  const getWorkflow = async (id: string) => {
    return await $fetch<FlowWorkflowDetail>(`/workflows/${id}`, { baseURL })
  }

  const validateWorkflow = async (id: string) => {
    return await $fetch<FlowValidationResult>(`/workflows/${id}/validate`, {
      baseURL,
      method: 'POST'
    })
  }

  const dryRunWorkflow = async (id: string, variables: Record<string, string> = {}) => {
    return await $fetch<FlowDryRunResult>(`/workflows/${id}/dry-run`, {
      baseURL,
      method: 'POST',
      body: { variables }
    })
  }

  const createRun = async (workflowId: string, variables: Record<string, string> = {}) => {
    return await $fetch<FlowRun>('/runs', {
      baseURL,
      method: 'POST',
      body: { workflowId, variables }
    })
  }

  const getRun = async (id: string) => {
    return await $fetch<FlowRun>(`/runs/${id}`, { baseURL })
  }

  const advanceRun = async (id: string) => {
    return await $fetch<FlowAdvanceRunResult>(`/runs/${id}/advance`, {
      baseURL,
      method: 'POST'
    })
  }

  const dispatchRunStep = async (id: string, payload: { sessionId?: string, note?: string }) => {
    return await $fetch<FlowStepDispatchResult>(`/runs/${id}/dispatch-step`, {
      baseURL,
      method: 'POST',
      body: payload
    })
  }

  const completeRunStep = async (id: string, payload: { summary: string, checkpointLabel?: string }) => {
    return await $fetch<FlowStepCompletionResult>(`/runs/${id}/complete-step`, {
      baseURL,
      method: 'POST',
      body: payload
    })
  }

  const failRunStep = async (id: string, payload: { reason: string }) => {
    return await $fetch<FlowStepFailureResult>(`/runs/${id}/fail-step`, {
      baseURL,
      method: 'POST',
      body: payload
    })
  }

  return {
    getWorkflows,
    getWorkflow,
    getRuns,
    getRun,
    advanceRun,
    dispatchRunStep,
    completeRunStep,
    failRunStep,
    validateWorkflow,
    dryRunWorkflow,
    createRun
  }
}
