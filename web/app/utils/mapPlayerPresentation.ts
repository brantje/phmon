export const OTHER_PLAYER_ICON =
  '/game-assets/interface/minimap/mm_sign_otherplayer.png'

export const playerAliveLabel = (dead?: boolean | null) => {
  if (dead === true) return 'Dead'
  if (dead === false) return 'Alive'
  return 'Unknown'
}
