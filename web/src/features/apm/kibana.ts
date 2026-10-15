// Kibana APM 화면으로 가는 링크. KIBANA_URL 이 설정돼 있을 때만 쓴다.

const env = (environment?: string) => (environment ? `environment=${encodeURIComponent(environment)}` : '')

export const kibanaService = (base: string, service: string, environment?: string) =>
  `${base}/app/apm/services/${encodeURIComponent(service)}/overview?${env(environment)}`

export const kibanaTransaction = (base: string, service: string, name: string, environment?: string) =>
  `${base}/app/apm/services/${encodeURIComponent(service)}/transactions/view?transactionName=${encodeURIComponent(name)}&transactionType=request&${env(environment)}`

export const kibanaError = (base: string, service: string, groupingKey: string, environment?: string) =>
  `${base}/app/apm/services/${encodeURIComponent(service)}/errors/${encodeURIComponent(groupingKey)}?${env(environment)}`
