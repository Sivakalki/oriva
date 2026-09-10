import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import * as api from "./endpoints"

export const useJobs = () => useQuery({ queryKey: ["jobs"], queryFn: api.listJobs })
export const useCandidates = () =>
  useQuery({ queryKey: ["candidates"], queryFn: api.listCandidates })

export const useInterviews = (state?: string) =>
  useQuery({ queryKey: ["interviews", state ?? "all"], queryFn: () => api.listInterviews(state) })

export const useInterview = (id: string) =>
  useQuery({ queryKey: ["interview", id], queryFn: () => api.getInterview(id) })

export const useSessionStates = () =>
  useQuery({ queryKey: ["session-states"], queryFn: api.sessionStates, staleTime: 5 * 60_000 })

export function useCreateJob() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ title, description }: { title: string; description: string }) =>
      api.createJob(title, description),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["jobs"] }),
  })
}

export function useCreateCandidate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (v: { email: string; name: string; resume_text: string }) =>
      api.createCandidate(v.email, v.name, v.resume_text),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["candidates"] }),
  })
}

export function useScheduleInterview() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: api.scheduleInterview,
    onSuccess: () => qc.invalidateQueries({ queryKey: ["interviews"] }),
  })
}

export function useAdvanceState(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ to_state, reason }: { to_state: string; reason: string }) =>
      api.advanceState(id, to_state, reason),
    onSuccess: (d) => {
      qc.setQueryData(["interview", id], d)
      qc.invalidateQueries({ queryKey: ["interviews"] })
    },
  })
}

export const useJoinStatus = (token: string) =>
  useQuery({
    queryKey: ["join", token],
    queryFn: () => api.joinStatus(token),
    refetchInterval: 15_000,
    retry: false,
  })
