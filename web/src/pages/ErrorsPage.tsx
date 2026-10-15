import { ServiceTabs } from '../features/apm/components/ServiceTabs'
import { ApmRangeNote } from '../features/apm/components/ApmRangeNote'
import { ErrorKpis } from '../features/apm/components/ErrorKpis'
import { ErrorRateChart } from '../features/apm/components/ErrorRateChart'
import { ErrorGroupTable } from '../features/apm/components/ErrorGroupTable'
import { useServiceFilter } from '../features/apm/serviceFilter'
import { useRangeFilter } from '../shared/filters/range'

// 에러: 에러율 추이와 원인별 에러 그룹. 화면 배치만 하고 데이터는 apm 기능이 가져온다.
export function ErrorsPage() {
  const { range } = useRangeFilter()
  const { service } = useServiceFilter()

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-lg font-semibold">에러</h1>
        <ServiceTabs />
      </div>
      <ApmRangeNote range={range} />
      <ErrorKpis span={range.apm} service={service} />
      <ErrorRateChart span={range.apm} service={service} />
      <ErrorGroupTable span={range.apm} service={service} />
    </>
  )
}
