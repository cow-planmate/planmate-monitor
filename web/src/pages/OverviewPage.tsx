import { StatsKpis } from '../features/stats/components/StatsKpis'
import { GrowthChart } from '../features/stats/components/GrowthChart'
import { ApmKpis } from '../features/apm/components/ApmKpis'
import { ServiceTable } from '../features/apm/components/ServiceTable'
import { useRangeFilter } from '../shared/filters/range'

// 개요: "지금 서비스가 괜찮은가 + 사용자 지표 요약". 화면 배치만 하고 데이터는 각 기능이 가져온다.
export function OverviewPage() {
  const { range } = useRangeFilter()

  return (
    <>
      <h1 className="text-lg font-semibold">개요</h1>
      <StatsKpis />
      <ApmKpis />
      <div className="grid gap-4 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <GrowthChart span={range.stats} />
        </div>
        <ServiceTable />
      </div>
    </>
  )
}
