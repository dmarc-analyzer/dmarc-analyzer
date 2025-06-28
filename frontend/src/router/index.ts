import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHistory } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Domains',
    component: () => import('@/views/DomainsView.vue'),
  },
  {
    path: '/report/:domain',
    name: 'Report',
    component: () => import('@/views/ReportView.vue'),
  },
  {
    path: '/report/:domain/:start/:end',
    name: 'ReportWithDates',
    component: () => import('@/views/ReportView.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

export default router
