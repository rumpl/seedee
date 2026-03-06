import { Status } from "@/gen/seedee/v1/seedee_pb"

export function statusIcon(status: Status): string {
  switch (status) {
    case Status.PENDING:
      return "⏳"
    case Status.RUNNING:
      return "▶️"
    case Status.SUCCESS:
      return "✅"
    case Status.FAILED:
      return "❌"
    case Status.SKIPPED:
      return "⏭️"
    case Status.CANCELED:
      return "🚫"
    default:
      return "❓"
  }
}
