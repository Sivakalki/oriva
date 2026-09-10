import { createBrowserRouter } from "react-router-dom"

import { RequireAuth } from "@/auth/RequireAuth"
import { AppShell } from "@/components/AppShell"
import { Candidates } from "@/pages/Candidates"
import { Dashboard } from "@/pages/Dashboard"
import { InterviewDetail } from "@/pages/InterviewDetail"
import { Jobs } from "@/pages/Jobs"
import { Join } from "@/pages/Join"
import { Login } from "@/pages/Login"
import { NotFound } from "@/pages/NotFound"
import { ScheduleInterview } from "@/pages/ScheduleInterview"

export const router = createBrowserRouter([
  { path: "/login", element: <Login /> },
  { path: "/join/:token", element: <Join /> },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppShell />,
        children: [
          { index: true, element: <Dashboard /> },
          { path: "jobs", element: <Jobs /> },
          { path: "candidates", element: <Candidates /> },
          { path: "schedule", element: <ScheduleInterview /> },
          { path: "interviews/:id", element: <InterviewDetail /> },
        ],
      },
    ],
  },
  { path: "*", element: <NotFound /> },
])
