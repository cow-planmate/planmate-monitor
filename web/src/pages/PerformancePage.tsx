import { ServiceTabs } from '../features/apm/components/ServiceTabs'
import { ApmRangeNote } from '../features/apm/components/ApmRangeNote'
import { LatencyChart } from '../features/apm/components/LatencyChart'
import { EndpointTable } from '../features/apm/components/EndpointTable'
import { JvmCharts } from '../features/apm/components/JvmCharts'
import { useServiceFilter } from '../features/apm/serviceFilter'
import { useRangeFilter } from '../shared/filters/range'

// 성능: 응답시간·처리량, 느린 엔드포인트, JVM. 화면 배치만 하고 데이터는 apm 기능이 가져온다.
export function PerformancePage() {
  const { range } = useRangeFilter()
  const { service } = useServiceFilter()

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-lg font-semibold">성능</h1>
        <ServiceTabs />
      </div>
      <ApmRangeNote range={range} />
      <LatencyChart span={range.apm} service={service} />
      <EndpointTable span={range.apm} service={service} />
      <JvmCharts span={range.apm} service={service} />
    </>
  )
}
