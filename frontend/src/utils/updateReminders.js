const localDay = (milliseconds) => new Date(milliseconds).toDateString();


function update(episode, airingAt) {
  if (!(airingAt > 0)) return null;
  return { episode: episode || 0, airingAt, key: `${localDay(airingAt * 1000)}:${episode || 0}` };
}

// Retain observed updates and the next known broadcast. If the app is closed
// through a broadcast day, the saved schedule can still produce an unread badge.
export function rememberUpdates(records, airings, now = Date.now()) {
  const result = { ...records };
  for (const id of new Set([...Object.keys(records), ...Object.keys(airings)])) {
    const previous = records[id] || {};
    const record = { ...previous };
    // Older versions may have persisted a future airing as already updated.
    if (record.latest?.airingAt * 1000 > now) record.latest = null;
    const airing = airings[id];
    const next = update(airing?.next_episode, airing?.next_airing_at);
    let savedNext = previous.next;
    // Respect a corrected/postponed schedule for the same episode.
    if (next && savedNext?.episode === next.episode) savedNext = next;
    if (next && record.latest?.episode === next.episode && next.airingAt * 1000 > now) record.latest = null;
    for (const observed of [savedNext, update(airing?.today_episode, airing?.today_airing_at), next]) {
      const candidate = next && observed?.episode === next.episode ? next : observed;
      if (!candidate || candidate.airingAt * 1000 > now) continue;
      if (!record.latest || candidate.airingAt > record.latest.airingAt ||
          (candidate.airingAt === record.latest.airingAt && candidate.episode > record.latest.episode)) {
        record.latest = candidate;
      }
    }
    record.next = next || savedNext || null;
    result[id] = record;
  }
  return result;
}

export function hasUnreadUpdate(record) {
  const latest = record?.latest;
  if (!latest || record.seenKey === latest.key) return false;
  const day = localDay(latest.airingAt * 1000);
  if (record.seenKey === `${day}:*`) return false;
  return !(record.seenThrough && latest.airingAt * 1000 <= record.seenThrough);
}

export function readUpdate(record = {}, metadataReady = true, now = Date.now()) {
  return {
    ...record,
    seenKey: record.latest?.key || record.seenKey || '',
    // A click before metadata arrives also acknowledges older updates discovered
    // by that pending lookup, while retaining the last known update separately.
    seenThrough: metadataReady ? record.seenThrough || 0 : now,
  };
}
