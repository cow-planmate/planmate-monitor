import { createBrowserRouter } from 'react-router-dom'
import { AppLayout } from './layout/AppLayout'
import { OverviewPage } from '../pages/OverviewPage'
import { UsersPage } from '../pages/UsersPage'
import { PerformancePage } from '../pages/PerformancePage'
import { ErrorsPage } from '../pages/ErrorsPage'
import { ComingSoonPage } from '../pages/ComingSoonPage'

export const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      { index: true, element: <OverviewPage /> },
      { path: 'users', element: <UsersPage /> },
      { path: 'performance', element: <PerformancePage /> },
      { path: 'errors', element: <ErrorsPage /> },
      { path: '*', element: <ComingSoonPage title="페이지 없음" plan={['왼쪽 메뉴에서 화면을 고른다']} /> },
    ],
  },
])
