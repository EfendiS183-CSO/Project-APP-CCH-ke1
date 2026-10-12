import { createBrowserRouter } from 'react-router-dom'
import { AppShell } from './layouts/AppShell.jsx'
import { RequireAuth } from './guards/RequireAuth.jsx'
import LoginPage from '../features/auth/pages/LoginPage.jsx'
import GuestIntakePage from '../features/intake/submission/pages/GuestIntakePage.jsx'
import IntakeResultPage from '../features/intake/submission/pages/IntakeResultPage.jsx'
import ReviewQueuePage from '../features/intake/review/pages/ReviewQueuePage.jsx'
import ReviewDraftPage from '../features/intake/review/pages/ReviewDraftPage.jsx'
import { WorkspaceDashboard } from '../features/complaints/pages/WorkspaceDashboard.jsx'
import { OpenComplaintsPage } from '../features/complaints/pages/OpenComplaintsPage.jsx'
import { CompletedComplaintsPage } from '../features/complaints/pages/CompletedComplaintsPage.jsx'
import {
  ReceivedOpenComplaintsPage,
  SentOpenComplaintsPage,
} from '../features/complaints/pages/ReceivedSentComplaintsPages.jsx'
import { ComplaintDetailPage } from '../features/complaints/pages/ComplaintDetailPage.jsx'
import { FollowUpsWorkspacePage } from '../features/followups/pages/FollowUpsWorkspacePage.jsx'
import {
  OfficeFollowUpsPage,
  DueTodayFollowUpsPage,
} from '../features/followups/pages/FollowUpPages.jsx'

export const AppRoutes = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    children: [
      { index: true, element: <GuestIntakePage /> },
      { path: 'login', element: <LoginPage /> },
      { path: 'intake', element: <GuestIntakePage /> },
      { path: 'intake/result', element: <IntakeResultPage /> },
      {
        path: 'review',
        element: (
          <RequireAuth>
            <ReviewQueuePage />
          </RequireAuth>
        ),
      },
      {
        path: 'review/drafts/:id',
        element: (
          <RequireAuth>
            <ReviewDraftPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace',
        element: (
          <RequireAuth>
            <WorkspaceDashboard />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/follow-ups',
        element: (
          <RequireAuth>
            <FollowUpsWorkspacePage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/follow-ups/all',
        element: (
          <RequireAuth>
            <OfficeFollowUpsPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/follow-ups/due-today',
        element: (
          <RequireAuth>
            <DueTodayFollowUpsPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/complaints/open',
        element: (
          <RequireAuth>
            <OpenComplaintsPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/complaints/completed',
        element: (
          <RequireAuth>
            <CompletedComplaintsPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/complaints/received-open',
        element: (
          <RequireAuth>
            <ReceivedOpenComplaintsPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/complaints/sent-open',
        element: (
          <RequireAuth>
            <SentOpenComplaintsPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/complaints/open/:id',
        element: (
          <RequireAuth>
            <ComplaintDetailPage />
          </RequireAuth>
        ),
      },
      {
        path: 'workspace/complaints/completed/:id',
        element: (
          <RequireAuth>
            <ComplaintDetailPage completed />
          </RequireAuth>
        ),
      },
    ],
  },
])