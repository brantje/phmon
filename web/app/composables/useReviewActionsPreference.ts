/** Browser-local preference for reviewing a command before it is submitted. */
export function useReviewActionsPreference() {
  const reviewActions = useCookie<boolean>('phmon-review-actions', {
    default: () => false,
    sameSite: 'lax',
  })

  return computed({
    get: () => reviewActions.value === true,
    set: (value: boolean) => {
      reviewActions.value = value
    },
  })
}
