import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { useLogSearch } from './useLogSearch'

describe('useLogSearch', () => {
  it('点击搜索后统计、循环定位并暂停滚动', async () => {
    const rows = ref([{ message: 'boot ok' }, { message: 'timeout one' }, { message: 'TIMEOUT two' }])
    const scroll = vi.fn()
    const search = useLogSearch(rows, scroll)
    search.draft.value = 'timeout'
    search.search()
    expect(search.matches.value).toEqual([1, 2])
    expect(search.positionText.value).toBe('1 / 2')
    expect(search.follow.value).toBe(false)
    search.move(-1)
    expect(search.positionText.value).toBe('2 / 2')
    search.move(1)
    expect(search.positionText.value).toBe('1 / 2')
  })

  it('按纯文本安全分段，不把正则字符当表达式', () => {
    const search = useLogSearch(ref([{ message: 'value [a+b] found' }]), vi.fn())
    search.draft.value = '[a+b]'
    search.search()
    expect(search.segments('value [a+b] found')).toEqual([
      { text: 'value ', match: false },
      { text: '[a+b]', match: true },
      { text: ' found', match: false },
    ])
  })
})
