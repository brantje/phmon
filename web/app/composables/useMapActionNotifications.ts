import type { FanOutOperation } from '~/utils/commandFanOut'
import {
  collectMapActionNotifications,
  type MapActionNotification,
} from '~/utils/mapActionNotifications'

export function useMapActionNotifications(
  operations: () => FanOutOperation[],
  notify: (notification: MapActionNotification) => void,
) {
  const announced = new Map<string, string>()
  watch(
    operations,
    (current) => {
      for (const notification of collectMapActionNotifications(
        current,
        announced,
      ))
        notify(notification)
    },
    { deep: true },
  )
}
