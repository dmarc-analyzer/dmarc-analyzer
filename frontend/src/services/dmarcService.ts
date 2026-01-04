import type { AxiosResponse } from 'axios'
import type {
  DmarcChartResp,
  DmarcDetailResp,
  DmarcReportingDetail,
  DmarcReportingSummary,
  DomainStat,
  DomainSummaryResp,
  DomainSummaryCounts as OpenApiDomainSummaryCounts,
} from './openapi'
import { Configuration, DefaultApi } from './openapi'

export type DomainSummaryResponse = DomainSummaryResp
export type DomainDetailResponse = DmarcDetailResp
export type ChartDataResponse = DmarcChartResp
export type DetailReportRow = DmarcReportingDetail
export type SummaryEntry = DmarcReportingSummary
export type DomainSummaryCounts = OpenApiDomainSummaryCounts
export type { DomainStat }

const apiClient = new DefaultApi(
  new Configuration({
    basePath: '',
    baseOptions: {
      headers: {
        'Content-Type': 'application/json',
      },
    },
  }),
)

/**
 * DMARC Service API client
 */
export interface DmarcServiceInterface {
  getDomains: () => Promise<AxiosResponse<DomainStat[]>>
  getDomainSummary: (domain: string, startDate?: string, endDate?: string) => Promise<AxiosResponse<DomainSummaryResponse>>
  getDomainDetail: (domain: string, source: string, startDate?: string, endDate?: string, sourceType?: string) => Promise<AxiosResponse<DomainDetailResponse>>
  getChartData: (domain: string, startDate?: string, endDate?: string) => Promise<AxiosResponse<ChartDataResponse>>
}

const dmarcService: DmarcServiceInterface = {
  // Get list of domains
  getDomains(): Promise<AxiosResponse<DomainStat[]>> {
    return apiClient.handleDomainList()
  },

  // Get domain summary report
  getDomainSummary(domain: string, startDate?: string, endDate?: string): Promise<AxiosResponse<DomainSummaryResponse>> {
    return apiClient.handleDomainSummary(domain, startDate, endDate)
  },

  // Get domain detail report
  getDomainDetail(domain: string, source: string, startDate?: string, endDate?: string, sourceType: string = ''): Promise<AxiosResponse<DomainDetailResponse>> {
    const resolvedSourceType = sourceType || undefined
    return apiClient.handleDmarcDetail(domain, startDate, endDate, source, resolvedSourceType)
  },

  // Get chart data for domain
  getChartData(domain: string, startDate?: string, endDate?: string): Promise<AxiosResponse<ChartDataResponse>> {
    return apiClient.handleDmarcChart(domain, startDate, endDate)
  },
}

export default dmarcService
