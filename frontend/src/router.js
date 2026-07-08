import { createRouter, createWebHistory } from 'vue-router'
import DashboardPage from './pages/DashboardPage.vue'
import InterviewsPage from './pages/InterviewsPage.vue'
import InterviewDetailPage from './pages/InterviewDetailPage.vue'
import MemoryInboxPage from './pages/MemoryInboxPage.vue'
import CoachingPage from './pages/CoachingPage.vue'
import MockInterviewPage from './pages/MockInterviewPage.vue'
import EngineeringTracePage from './pages/EngineeringTracePage.vue'
import PracticeGoalsPage from './pages/PracticeGoalsPage.vue'

export const navItems = [
  { to: '/', label: 'Dashboard', exact: true },
  { to: '/interviews', label: 'Interviews' },
  { to: '/practice-goals', label: 'Practice Goals' },
  { to: '/memory', label: 'Memory Inbox' },
  { to: '/coaching', label: 'Coaching' },
  { to: '/mock', label: 'Mock Interview' },
  { to: '/trace', label: 'Engineering Trace' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardPage },
    { path: '/interviews', name: 'interviews', component: InterviewsPage },
    { path: '/interviews/:interviewId', name: 'interview-detail', component: InterviewDetailPage, props: true },
    { path: '/practice-goals', name: 'practice-goals', component: PracticeGoalsPage },
    { path: '/memory', name: 'memory', component: MemoryInboxPage },
    { path: '/coaching', name: 'coaching', component: CoachingPage },
    { path: '/mock', name: 'mock', component: MockInterviewPage },
    { path: '/trace', name: 'trace', component: EngineeringTracePage },
  ],
})
