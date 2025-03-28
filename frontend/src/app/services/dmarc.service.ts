import { HttpClient, HttpHeaders } from '@angular/common/http';
import { EventEmitter, Injectable, Pipe, PipeTransform } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { environment } from '../../environments/environment';
import { MatTableDataSource } from '@angular/material/table';

export class DomainDmarcDataProvider {
  public onData = new EventEmitter<void>();
  public onError = new EventEmitter<string>();
  public totalDataSource = new MatTableDataSource<IDMARCReportTotalRecord>();
  public summaryDataSource = new MatTableDataSource<IDMARCReportSummaryRecord>();

  constructor(
    public domain: string,
    public startDate: ISODateString,
    public endDate: ISODateString
  ) {
    this.domain = domain;
  }
}

export class DomainDmarcDetailDataProvider {
  public onData = new EventEmitter<void>();
  public onError = new EventEmitter<string>();
  public DetailDataSource = new MatTableDataSource<IDMARCReportDetailRecord>();

  constructor(
    public domain: string,
    public startDate: ISODateString,
    public endDate: ISODateString,
    public source: string
  ) { }
}

export interface IDMARCReportResponse {
  start_date: number;
  end_date: number;
  max_volume: number;
  report_providers: string[];
  domain_summary_counts: IDMARCReportTotalRecord;
  summary: IDMARCReportSummaryRecord[];
  errorMessage?: string;
  domain: string;
}

export interface IDMARCReportDetailResponse {
  detail_rows: IDMARCReportDetailRecord[];
  errorMessage?: string;
}

export interface IDMARCReportTotalRecord {
  dkim_aligned_count: number;
  fully_aligned_count: number;
  message_count: number;
  spf_aligned_count: number;
}

export interface IDMARCReportSummaryRecord {
  dkim_aligned_count: number;
  fully_aligned_count: number;
  pass_count: number;
  source: string;
  source_type: string;
  spf_aligned_count: number;
  total_count: number;
}

export interface IDMARCReportDetailRecord {
  auth_dkim_domain: string[];
  auth_dkim_selector: string[];
  auth_dkim_result: string[];
  auth_spf_domain: string[];
  auth_spf_result: string[];
  auth_spf_scope: string[];
  country: string;
  disposition: string;
  source_domain: string;
  envelope_from: string;
  envelope_to: string;
  esp: string;
  eval_dkim: string;
  eval_spf: string;
  header_from: string;
  source_host: string;
  message_count: number;
  po_reason: string[];
  po_comment: string[];
  reverse_lookup: string[];
  source_ip: string;
}

type ISODateString = string;

@Pipe({ name: 'dashWhenEmptyString' })
export class DashWhenEmptyStringPipe implements PipeTransform {
  transform(value: string) {
    if (Array.isArray(value)) {
      return value.length === 0 ? '-' : value.join('\n');
    }
    return value === '' || value === '""' ? '-' : value;
  }
}

export interface IDmarcChart {
  chartdata: Array<IDaily>;
}

export interface IDaily {
  name: string;
  series: Array<IVolume>;
}

export interface IVolume {
  name: any;
  value: number;
}


@Injectable({
  providedIn: 'root'
})
export class DmarcService {
  public ChartDmarcResponse: IDmarcChart;
  constructor(
    private http: HttpClient
  ) { }

  getChartData(domain: string, startDate: ISODateString, endDate: ISODateString): Observable<IDmarcChart> {
    const data = new Subject<IDmarcChart>();
      this.http
        .get(`${environment.apiBaseUrl}/domains/${domain}/chart/dmarc`, {
          params: { start: startDate, end: endDate },
        })
        .subscribe(
          (response: any) => {
            this.ChartDmarcResponse = response;
            for (let j = 0; j < this.ChartDmarcResponse.chartdata[0].series.length; j++) {
              this.ChartDmarcResponse.chartdata[0].series[j].name = new Date(this.ChartDmarcResponse.chartdata[0].series[j].name);
              this.ChartDmarcResponse.chartdata[1].series[j].name = new Date(this.ChartDmarcResponse.chartdata[1].series[j].name);
            }
            data.next(this.ChartDmarcResponse);
          }, err => console.log(err)
        );
    return data.asObservable();
  }

  getDomainList() {
    const data = new Subject<any>(); // Use the appropriate type instead of any if available
      this.http.get(`${environment.apiBaseUrl}/domains`)
        .subscribe(
          (response: any) => {
            data.next(response); // Process the response as needed
          },
          err => {
            console.log(err);
            data.error(err); // Handle the error
          }
        );
    return data.asObservable();
  }

  getSummaryDataProvider(domain: string, startDate: ISODateString, endDate: ISODateString) {
    const dataProvider = new DomainDmarcDataProvider(domain, startDate, endDate);
      this.http
        .get(`${environment.apiBaseUrl}/domains/${domain}/report`, {
          params: { start: startDate, end: endDate },
        })
        .subscribe(
          (data: IDMARCReportResponse) => {
            if (data.errorMessage) { dataProvider.onError.emit(data.errorMessage); }
            dataProvider.totalDataSource.data = [data.domain_summary_counts];
            dataProvider.summaryDataSource.data = data.summary;
            dataProvider.domain = data.domain;
          },
          err => dataProvider.onError.emit('There was a problem processing this request'),
          () => dataProvider.onData.emit()
        );
    return dataProvider;
  }

  getDetailDataProvider(domainNav: string, domain: string, startDate: ISODateString, endDate: ISODateString, source: string, source_type: string) {
    const dataProvider = new DomainDmarcDetailDataProvider(domain, startDate, endDate, source);
      this.http
        .get(`${environment.apiBaseUrl}/domains/${domainNav}/report/detail`, {
          params: {
            source: source,
            source_type: source_type,
            start: startDate,
            end: endDate
          },
        })
        .subscribe(
          (data: any) => {
            if (data.errorMessage) { dataProvider.onError.emit(data.errorMessage); }
            dataProvider.DetailDataSource.data = data.detail_rows;
          },
          err => {
            dataProvider.onError.emit('There was a problem processing this request');
          },
          () => dataProvider.onData.emit()
        );
    return dataProvider;
  }
}
