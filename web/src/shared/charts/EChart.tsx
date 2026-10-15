import { useEffect, useRef } from 'react'
import * as echarts from 'echarts/core'
import { BarChart, LineChart } from 'echarts/charts'
import { DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsCoreOption } from 'echarts/core'

// 쓰는 차트 종류만 등록해서 번들을 줄인다. 새 종류가 필요하면 여기에 추가한다.
echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, CanvasRenderer])

interface EChartProps {
  option: EChartsCoreOption
  height?: number
}

// ECharts 인스턴스 수명과 크기 변화를 관리하는 얇은 래퍼. 옵션 구성은 각 차트 컴포넌트가 한다.
export function EChart({ option, height = 280 }: EChartProps) {
  const el = useRef<HTMLDivElement>(null)
  const chart = useRef<echarts.ECharts | null>(null)

  useEffect(() => {
    if (!el.current) return
    const c = echarts.init(el.current, undefined, { renderer: 'canvas' })
    chart.current = c
    const ro = new ResizeObserver(() => c.resize())
    ro.observe(el.current)
    return () => {
      ro.disconnect()
      c.dispose()
      chart.current = null
    }
  }, [])

  useEffect(() => {
    chart.current?.setOption(option, { notMerge: true })
  }, [option])

  return <div ref={el} style={{ height }} className="w-full" />
}

// CSS 토큰을 읽어 차트 색을 테마와 맞춘다.
export function themeColors() {
  const s = getComputedStyle(document.documentElement)
  const v = (name: string) => s.getPropertyValue(name).trim()
  return {
    text: v('--text'),
    muted: v('--text-muted'),
    border: v('--border'),
    surface: v('--surface-2'),
    series: [v('--accent'), v('--accent-2'), v('--ok'), v('--warn')],
  }
}
