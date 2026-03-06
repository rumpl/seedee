import { Badge } from "@/components/ui/8bit/badge"
import { Status } from "@/gen/seedee/v1/seedee_pb"
import { statusIcon } from "@/lib/status"

interface StatusBadgeProps {
  status: Status
}

const statusConfig: Record<
  Status,
  { label: string; className: string; pulse?: boolean }
> = {
  [Status.UNSPECIFIED]: {
    label: "Unknown",
    className:
      "bg-gray-200 text-gray-700 border-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600",
  },
  [Status.PENDING]: {
    label: "Pending",
    className:
      "bg-gray-200 text-gray-700 border-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600",
  },
  [Status.RUNNING]: {
    label: "Running",
    className:
      "bg-blue-200 text-blue-800 border-blue-300 dark:bg-blue-900 dark:text-blue-300 dark:border-blue-700",
    pulse: true,
  },
  [Status.SUCCESS]: {
    label: "Success",
    className:
      "bg-green-200 text-green-800 border-green-300 dark:bg-green-900 dark:text-green-300 dark:border-green-700",
  },
  [Status.FAILED]: {
    label: "Failed",
    className:
      "bg-red-200 text-red-800 border-red-300 dark:bg-red-900 dark:text-red-300 dark:border-red-700",
  },
  [Status.SKIPPED]: {
    label: "Skipped",
    className:
      "bg-yellow-200 text-yellow-800 border-yellow-300 dark:bg-yellow-900 dark:text-yellow-300 dark:border-yellow-700",
  },
  [Status.CANCELED]: {
    label: "Canceled",
    className:
      "bg-orange-200 text-orange-800 border-orange-300 dark:bg-orange-900 dark:text-orange-300 dark:border-orange-700",
  },
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const config = statusConfig[status] ?? statusConfig[Status.UNSPECIFIED]

  return (
    <Badge
      className={`transition-all duration-300 ${config.className} ${config.pulse ? "animate-pulse" : ""}`}
      aria-label={`Status: ${config.label}`}
    >
      <span className="mr-1" aria-hidden="true">
        {statusIcon(status)}
      </span>
      {config.label}
    </Badge>
  )
}
