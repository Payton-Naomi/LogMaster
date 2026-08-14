import { computed, nextTick, ref } from 'vue'

export function useLogSearch(rows, scrollToIndex) {
  const draft = ref('')
  const term = ref('')
  const current = ref(-1)
  const follow = ref(true)
  const matches = computed(() => {
    if (!term.value) return []
    const needle = term.value.toLocaleLowerCase()
    const result = []
    rows.value.forEach((row, index) => { if ((row.message || row.text || '').toLocaleLowerCase().includes(needle)) result.push(index) })
    return result
  })
  const positionText = computed(() => matches.value.length ? `${current.value + 1} / ${matches.value.length}` : '0 / 0')
  function locate() { const index = matches.value[current.value]; if (index === undefined) return; nextTick(() => scrollToIndex?.(index)) }
  function search() { term.value = draft.value.trim(); current.value = matches.value.length ? 0 : -1; follow.value = false; locate() }
  function move(delta) { if (!matches.value.length) return; current.value = (current.value + delta + matches.value.length) % matches.value.length; follow.value = false; locate() }
  function resume() { follow.value = true; current.value = -1; scrollToIndex?.(Math.max(0, rows.value.length - 1)) }
  function clearSearch() { draft.value = ''; term.value = ''; current.value = -1 }
  function segments(text) { if (!term.value) return [{ text, match: false }]; const escaped = term.value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'); return String(text).split(new RegExp(`(${escaped})`, 'gi')).filter(Boolean).map((part) => ({ text: part, match: part.toLocaleLowerCase() === term.value.toLocaleLowerCase() })) }
  return { draft, term, current, follow, matches, positionText, search, move, resume, clearSearch, segments }
}
