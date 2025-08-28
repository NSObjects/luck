import axios from 'axios'

export interface BandRange { low: [number, number]; mid: [number, number]; high: [number, number] }
export interface BandTpl { vals: [number, number, number] }
export interface StartBucket { From: number; To: number; Count: number }
export interface BandRangeGo { LowLo: number; LowHi: number; MidLo: number; MidHi: number; HighLo: number; HighHi: number }
export interface GenConfig {
  Mode: number
  Animal: number
  Birthday: string

  GenerateCount: number
  BudgetYuan: number

  RedFilter: number[]
  BlueFilter: number[]
  FixedRed: number[]

  FMode: number
  FixedPerTicket: number

  MaxOverlapRed: number
  UsePerNumberCap: boolean

  StartBuckets: StartBucket[]
  MaxPerAnchor: number

  Bands: BandRangeGo
  BandTemplates: number[][]  // [low, mid, high]，三者和=6
  TemplateRepeat: number
}

// 兼容别名（旧代码仍可用 getConfig/putConfig）
export type AppConfig = GenConfig
export const getGenConfig = () => http.get<GenConfig>('/api/config').then(r => r.data)
export const putGenConfig = (cfg: Partial<GenConfig>) => http.put('/api/config', cfg).then(r => r.data)

export interface Combo { reds: number[]; blue: number }
export interface Stats {
  red_freq: Record<number, number>
  blue_freq: Record<number, number>
  band_share: Record<'low'|'mid'|'high', number>
  odd_even: Record<'odd'|'even', number>
  high_low: Record<'low'|'high', number>
}
export interface GenerateResponse { combos: Combo[]; stats?: Stats }
export interface HistorySummary { total_combos: number }
export interface Heatmap { redMatrix: number[][]; blueVector: number[] }
export interface HotCold {
  redFreq: Record<number, number>
  blueFreq: Record<number, number>
  topHotRed: [number, number][]
  topColdRed: [number, number][]
  avgGapRed: Record<number, number>
  maxGapRed: Record<number, number>
  MA33: number[]
}
export interface Summary {
  odd: number; even: number; low: number; high: number;
  area: [number, number, number];
  sumMin: number; sumMax: number; sumAvg: number;
  consecLenDist: Record<number, number>;
  chiSquare: number; entropy: number;
}

const http = axios.create({
  // baseURL: '/',
  timeout: 15000,
})

export const getConfig = () => http.get<AppConfig>('/api/config').then(r => r.data)
export const putConfig = (cfg: Partial<AppConfig>) => http.put('/api/config', cfg).then(r => r.data)
export const generate = (payload: { override: boolean; config?: Partial<AppConfig> }) =>
  http.post<GenerateResponse>('/api/generate', payload).then(r => r.data)

export const uploadHistory = (file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  return http.post('/api/history/upload', fd).then(r => r.data)
}
export const getHistorySummary = () => http.get<HistorySummary>('/api/history/summary').then(r => r.data)

export const getAnalysisHeatmap = (window = 100) =>
  http.get<Heatmap>('/api/analysis/heatmap', { params: { window } }).then(r => r.data)
export const getAnalysisHot = (window = 50) =>
  http.get<HotCold>('/api/analysis/hot', { params: { window } }).then(r => r.data)
export const getAnalysisSummary = () =>
  http.get<Summary>('/api/analysis/summary').then(r => r.data)

export interface Draw {
    issue: string
    draw_date: string
    reds: number[]
    blue: number
    source?: string
    fetched_at?: string
  }
  
export const getLatestDraw = () =>
    http.get<Draw>('/api/draw/latest').then(r => r.data)
  
// optional future endpoint
// export const getLatestDraw = () => http.get<Draw>('/api/draw/latest').then(r => r.data)
