// API 호출 공통 처리. 화면과 API 가 같은 주소라 항상 /api 상대경로를 쓴다.
// 서버 에러는 { "error": "..." } 형식으로 오므로 그 메시지를 ApiError 로 올린다.

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type Params = Record<string, string | number | undefined>

export async function apiGet<T>(path: string, params?: Params, signal?: AbortSignal): Promise<T> {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v !== undefined) qs.set(k, String(v))
  }
  const url = qs.size > 0 ? `${path}?${qs}` : path

  const res = await fetch(url, { signal, headers: { Accept: 'application/json' } })
  if (!res.ok) {
    let message = `요청 실패 (${res.status})`
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // 본문이 JSON 이 아니면(예: Access 로그인 페이지) 상태 코드만 알린다.
    }
    throw new ApiError(res.status, message)
  }
  return (await res.json()) as T
}
