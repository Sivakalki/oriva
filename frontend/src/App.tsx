import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { RouterProvider } from "react-router-dom"
import { Toaster } from "sonner"

import { AuthProvider } from "@/auth/AuthProvider"
import { router } from "@/routes"
import { ThemeProvider } from "@/theme/ThemeProvider"
import { useTheme } from "@/theme/useTheme"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
})

function ThemedToaster() {
  const { resolvedTheme } = useTheme()
  return <Toaster richColors position="top-right" theme={resolvedTheme} />
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuthProvider>
          <RouterProvider router={router} />
          <ThemedToaster />
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}
