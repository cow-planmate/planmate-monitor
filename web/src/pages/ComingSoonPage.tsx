import { Panel } from '../shared/ui/Panel'

// 1단계 골격에서는 개요만 만든다. 나머지 화면은 자리만 잡아 둔다.
export function ComingSoonPage({ title, plan }: { title: string; plan: string[] }) {
  return (
    <>
      <h1 className="text-lg font-semibold">{title}</h1>
      <Panel title="준비 중" subtitle="다음 단계에서 채울 내용">
        <ul className="list-disc space-y-1 pl-5 text-sm text-muted">
          {plan.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      </Panel>
    </>
  )
}
