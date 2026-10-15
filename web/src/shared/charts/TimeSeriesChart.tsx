import { useMemo } from 'react'
import { EChart, themeColors } from './EChart'
import { formatBucket, formatInt } from '../lib/format'

export interface ChartSeries {
  name: string
  type: 'bar' | 'line'
  data: (number | null)[] // buckets 와 같은 길이. null 은 "값 없음"(선이 끊긴다)
  axis?: 0 | 1 // 1 이면 오른쪽 보조 축
  stack?: string // 같은 값끼리 쌓는다
  color?: string // CSS 토큰 이름(예: '--danger'). 생략하면 순서대로 팔레트 색
}

interface Props {
  buckets: string[]
  series: ChartSeries[]
  interval: string
  height?: number
  format?: (v: number) => string // 왼쪽 축·툴팁 단위
  format2?: (v: number) => string // 오른쪽 축 단위
}

// 시간 버킷 차트. 모든 시리즈는 같은 버킷 목록을 쓴다(서버가 빈 구간을 0 또는 null 로 채워 준다).
export function TimeSeriesChart({ buckets, series, interval, height, format = formatInt, format2 }: Props) {
  const option = useMemo(() => {
    const c = themeColors()
    const css = getComputedStyle(document.documentElement)
    const hasAxis2 = series.some((s) => s.axis === 1)
    const axisStyle = {
      type: 'value',
      splitLine: { lineStyle: { color: c.border, opacity: 0.5 } },
      axisLabel: { color: c.muted },
    }
    return {
      color: c.series,
      grid: { left: 8, right: 16, top: 36, bottom: 8, containLabel: true },
      legend: { top: 0, right: 0, textStyle: { color: c.muted }, itemWidth: 12, itemHeight: 8 },
      tooltip: {
        trigger: 'axis',
        backgroundColor: c.surface,
        borderColor: c.border,
        textStyle: { color: c.text },
      },
      xAxis: {
        type: 'category',
        data: buckets.map((t) => formatBucket(t, interval)),
        axisLine: { lineStyle: { color: c.border } },
        axisLabel: { color: c.muted, hideOverlap: true },
        axisTick: { show: false },
      },
      yAxis: [
        { ...axisStyle, axisLabel: { ...axisStyle.axisLabel, formatter: format } },
        ...(hasAxis2
          ? [{ ...axisStyle, splitLine: { show: false }, axisLabel: { ...axisStyle.axisLabel, formatter: format2 ?? format } }]
          : []),
      ],
      series: series.map((s) => {
        const fmt = s.axis === 1 ? (format2 ?? format) : format
        return {
          name: s.name,
          type: s.type,
          data: s.data,
          yAxisIndex: s.axis ?? 0,
          stack: s.stack,
          barMaxWidth: 18,
          smooth: s.type === 'line',
          symbol: 'none',
          connectNulls: false,
          emphasis: { focus: 'series' },
          tooltip: { valueFormatter: (v: number | null) => (v == null ? '—' : fmt(v)) },
          ...(s.color ? { itemStyle: { color: css.getPropertyValue(s.color).trim() } } : {}),
        }
      }),
    }
  }, [buckets, series, interval, format, format2])

  return <EChart option={option} height={height} />
}
