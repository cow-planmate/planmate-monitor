// 서버 httpapi.Meta 와 1:1 로 맞춘다. 비밀값 없는 서버 설정이다.
export interface Meta {
  environment: string
  kibanaUrl?: string
  timeZone: string
}
