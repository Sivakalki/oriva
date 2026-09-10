import { Link } from "react-router-dom"

export function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-2 text-center">
      <p className="text-2xl font-semibold">Page not found</p>
      <Link to="/" className="text-sm underline">
        Back to interviews
      </Link>
    </div>
  )
}
