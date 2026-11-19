// Mirrors phaseForSecond in internal/dashboard/unit_composition.go, so the events list and the composition
// bars split a game the same way: a game whose mid game never ended finished in the mid game.
export const gamePhaseFor = (second, earlyEnd, midEnd) => {
  if (earlyEnd <= 0 && midEnd <= 0) return 'early';
  if (earlyEnd <= 0) return second < midEnd ? 'early' : 'late';
  if (second < earlyEnd) return 'early';
  if (midEnd <= 0 || second < midEnd) return 'mid';
  return 'late';
};

export const gamePhaseStarts = (earlyEnd, midEnd) => {
  const starts = {};
  if (earlyEnd > 0) starts.mid = earlyEnd;
  if (midEnd > 0) starts.late = midEnd;
  return starts;
};
