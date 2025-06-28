import type { AxiosInstance, AxiosResponse } from 'axios'
import axios from 'axios'

// Create axios instance with base URL
const apiClient: AxiosInstance = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

// Type definitions for DMARC API responses

export interface CountEntry {
  total_count: number
  pass_count: number
  spf_aligned_count: number
  dkim_aligned_count: number
  fully_aligned_count: number
}

/**
 * Individual summary entry for a source in domain summary report
 */
export interface SummaryEntry extends CountEntry {
  source: string
  source_type: 'ESP' | 'SourceIP' | string
}

/**
 * Domain summary counts aggregation
 */
export interface DomainSummaryCounts extends CountEntry {
}

/**
 * Complete domain summary report response
 */
export interface DomainSummaryResponse {
  summary: SummaryEntry[]
  domain_summary_counts: DomainSummaryCounts
  start_date: string
  end_date: string
  domain: string
}

/**
 * Chart data point for time series
 */
export interface ChartDataPoint {
  name: string | number // Can be timestamp or date string
  value: number
}

/**
 * Chart data series
 */
export interface ChartDataSeries {
  name: 'pass' | 'fail' | 'total'
  series: ChartDataPoint[]
}

/**
 * Chart data response structure
 */
export interface ChartDataResponse {
  chartdata?: ChartDataSeries[]
  chart_data?: {
    dates: string[]
    pass: number[]
    fail: number[]
  }
}

/**
 * Detail report row entry
 */
export interface DetailReportRow {
  message_count: number
  report_org_name: string
  source_ip: string
  esp: string
  source_domain: string
  source_host: string
  reverse_lookup: string | null
  country: string
  disposition: string
  eval_dkim: string
  eval_spf: string
  header_from: string
  envelope_from: string
  envelope_to: string
  auth_dkim_domain: string[]
  auth_dkim_selector: string[]
  auth_dkim_result: string[]
  auth_spf_domain: string[]
  auth_spf_scope: string[]
  auth_spf_result: string[]
  po_reason: string[]
  po_comment: string[]
}

/**
 * Domain detail report response
 */
export interface DomainDetailResponse {
  detail_rows: DetailReportRow[]
  domain: string
  source: string
  start_date?: string
  end_date?: string
  source_type?: string
}

/**
 * Domains list response - direct array of domain names
 */
export type DomainsResponse = string[]

/**
 * DMARC Service API client
 */
export interface DmarcServiceInterface {
  getDomains: () => Promise<AxiosResponse<DomainsResponse>>
  getDomainSummary: (domain: string, startDate?: string, endDate?: string) => Promise<AxiosResponse<DomainSummaryResponse>>
  getDomainDetail: (domain: string, source: string, startDate?: string, endDate?: string, sourceType?: string) => Promise<AxiosResponse<DomainDetailResponse>>
  getChartData: (domain: string, startDate?: string, endDate?: string) => Promise<AxiosResponse<ChartDataResponse>>
}

const dmarcService: DmarcServiceInterface = {
  // Get list of domains
  getDomains(): Promise<AxiosResponse<DomainsResponse>> {
    return apiClient.get('/domains')
  },

  // Get domain summary report
  getDomainSummary(domain: string, startDate?: string, endDate?: string): Promise<AxiosResponse<DomainSummaryResponse>> {
    if (startDate && endDate) {
      return apiClient.get(`/domains/${domain}/report?start=${startDate}&end=${endDate}`)
    }
    return apiClient.get(`/domains/${domain}/report`)
  },

  // Get domain detail report
  getDomainDetail(domain: string, source: string, startDate?: string, endDate?: string, sourceType: string = ''): Promise<AxiosResponse<DomainDetailResponse>> {
    let url = `/domains/${domain}/report/detail?source=${source}`

    if (sourceType) {
      url += `&source_type=${sourceType}`
    }

    if (startDate && endDate) {
      url += `&start=${startDate}&end=${endDate}`
    }

    return apiClient.get(url)
  },

  // Get chart data for domain
  getChartData(domain: string, startDate?: string, endDate?: string): Promise<AxiosResponse<ChartDataResponse>> {
    if (startDate && endDate) {
      return apiClient.get(`/domains/${domain}/chart/dmarc?start=${startDate}&end=${endDate}`)
    }
    return apiClient.get(`/domains/${domain}/chart/dmarc`)
  },
}

export default dmarcService
