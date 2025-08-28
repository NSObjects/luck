<template>
  <div class="page">
    <!-- 左侧：配置面板（UI 不变） -->
    <ConfigPanel v-model="config" @apply="onApply" @reset="resetDefault" class="config"/>

    <!-- 右侧：主区 -->
    <main class="main">
      <!-- 顶部工具栏 -->
      <div class="toolbar">
        <div class="latest" v-if="latest">
          <span class="badge">最新</span>
          <b>{{ latest.issue }}</b>
          <span class="sep">·</span>
          <span>{{ latest.draw_date }}</span>
          <span class="sep">·</span>
          <span class="nums">
            <i v-for="n in latest.reds" :key="'lr'+n" class="ball red">{{ pad(n) }}</i>
            <i class="ball blue">{{ pad(latest.blue) }}</i>
          </span>
          <button class="ico" title="刷新" @click="fetchLatest">⟳</button>
        </div>

        <div class="flex-gap"></div>

        <div class="uploader" :class="{ dragover }"
             @dragover.prevent="dragover=true" @dragleave="dragover=false" @drop.prevent="onDrop">
          <input ref="fileInp" type="file" accept=".xlsx,.xls" @change="onPick" hidden />
          <button class="btn ghost" @click="pickFile">上传历史 Excel</button>
          <label class="ck"><input type="checkbox" v-model="replaceImport" /> 覆盖导入</label>
        </div>
      </div>

      <!-- 标签栏 -->
      <div class="tabs">
        <button v-for="t in tabs" :key="t" :class="['tab', { active: t===activeTab }]" @click="activeTab=t">{{ t }}</button>
        <div class="spacer"></div>
        <button class="btn" @click="onGenerate" :disabled="loadingGen">
          {{ loadingGen ? '生成中...' : '立即生成' }}
        </button>
      </div>

      <!-- 生成 -->
      <section class="panel stable" v-show="activeTab==='生成'">
        <div class="gen-head">
          <div class="hint">已生成 <b>{{ combos.length }}</b> 注</div>
          <div class="ops">
            <button class="btn ghost" :disabled="!combos.length" @click="copyCombos">复制</button>
            <button class="btn ghost" :disabled="!combos.length" @click="exportCSV">导出 CSV</button>
          </div>
        </div>

        <div v-if="loadingGen" class="loading">正在生成号码...</div>
        <div v-else-if="combos.length === 0" class="empty">点击右上角 “立即生成” 试试（快捷键 G）</div>

        <div v-else class="grid-cards">
          <div v-for="(c, idx) in combos" :key="idx" class="card">
            <div class="card-h"><span># {{ idx+1 }}</span></div>
            <div class="nums">
              <i v-for="n in c.reds" :key="'r'+idx+n" class="ball red">{{ pad(n) }}</i>
              <i class="ball blue">{{ pad(c.blue) }}</i>
            </div>
          </div>
        </div>
      </section>

      <!-- 热度：组件 -->
      <HotPanel v-show="activeTab==='热度'" :api="API" :initial-window="50" class="panel stable" />

      <!-- 热力图：组件 -->
      <HeatmapPanel v-show="activeTab==='热力图'" :api="API" :initial-window="100" class="panel stable" />

      <!-- 汇总（占位） -->
      <section class="panel stable" v-show="activeTab==='汇总'">
        <div class="loading">后续可接后端 /analysis/summary 在这里展示。</div>
      </section>

      <transition name="fade"><div v-if="toast" class="toast">{{ toast }}</div></transition>
    </main>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import ConfigPanel from '../components/ConfigPanel.vue'
import HotPanel from '../components/HotPanel.vue'
import HeatmapPanel from '../components/HeatmapPanel.vue'

const API = import.meta.env.VITE_API_BASE || `${location.origin}/api`

/* ---------------- 初始配置（UI 用；保持原展示不变） ---------------- */
const configDefault = () => ({
  Count: 10,
  Mode: 'mixed',           // 'random' | 'zodiac' | 'birthday' | 'mixed'
  Animal: 'Dog',           // 'Rat'..'Pig'
  Birthday: '1991-05-28',

  RedFilter: [], BlueFilter: [], FixedRed: [],
  FixedMode: 'rotate',     // 'always' | 'rotate'
  FixedPerTicket: 2,
  MaxOverlapRed: 3,
  UsePerNumberCap: true,

  Bands: { Low: [1,11], Mid: [12,22], High: [23,33] },
  BandTemplates: [
    {Vals:[2,2,2]}, {Vals:[2,3,1]}, {Vals:[3,2,1]},
    {Vals:[1,2,3]}, {Vals:[1,3,2]}
  ],
  TemplateRepeat: 2,

  // 后端也需要但 UI 不必展示的字段（给默认值）
  BudgetYuan: 0,
  StartBuckets: [
    { From: 1,  To: 10, Count: 3 },
    { From: 11, To: 18, Count: 2 },
    { From: 19, To: 32, Count: 2 },
  ],
  MaxPerAnchor: 1,

  // 前端专用（不会发到后端）
  UseAPISource: false, APIProvider: 'jisu', APIKey: '',
})
const config = ref(configDefault())

/* ---------------- 本地持久化 ---------------- */
const LS_KEY = 'ssq_config_v1'
try {
  const saved = localStorage.getItem(LS_KEY)
  if (saved) config.value = JSON.parse(saved)
} catch {}
watch(config, v => {
  try { localStorage.setItem(LS_KEY, JSON.stringify(v)) } catch {}
}, { deep: true })

/* ---------------- 页面状态 ---------------- */
const tabs = ['生成','热度','热力图','汇总']
const activeTab = ref('生成')

const loadingGen = ref(false)
const combos = ref([])
const stats = ref(null)
const latest = ref(null)

const fileInp = ref(null)
const replaceImport = ref(false)
const dragover = ref(false)
function pickFile(){ if(fileInp.value) fileInp.value.click() }

const toast = ref('')
let toastTimer=null
function showToast(m){ clearTimeout(toastTimer); toast.value=m; toastTimer=setTimeout(()=>toast.value='',1800) }

function onApply(v){ config.value=v; showToast('配置已应用') }
function resetDefault(){ config.value=configDefault(); showToast('已重置为默认配置') }

/* ---------------- 前端 → 后端 Config 类型转换（关键部分） ---------------- */
// 前端字符串 → 后端枚举数字
const ModeEnum = { random:0, zodiac:1, birthday:2, mixed:3 }
const FixedEnum = { always:0, rotate:1 }
const ZodiacEnum = {
  Rat:1, Ox:2, Tiger:3, Rabbit:4, Dragon:5, Snake:6,
  Horse:7, Goat:8, Monkey:9, Rooster:10, Dog:11, Pig:12
}
function fixRange(r=[1,33]) {
  let [lo, hi] = r
  lo = Math.max(1, Math.min(33, Number(lo||1)))
  hi = Math.max(1, Math.min(33, Number(hi||33)))
  if (lo > hi) [lo, hi] = [hi, lo]
  return [lo, hi]
}
function normalizeTpl(t) {
  const v = (t?.Vals || [0,0,0]).map(x => Math.max(0, Math.min(6, Number(x||0))))
  const sum = v[0]+v[1]+v[2]
  if (sum !== 6) v[1] = Math.max(0, Math.min(6, v[1] + (6 - sum))) // 中段兜底到 6
  return [v[0], v[1], v[2]]
}
function toBackendConfig(c) {
  const [lLo,lHi] = fixRange(c.Bands?.Low || [1,11])
  const [mLo,mHi] = fixRange(c.Bands?.Mid || [12,22])
  const [hLo,hHi] = fixRange(c.Bands?.High || [23,33])
  return {
    // 选号策略
    Mode: ModeEnum[c.Mode] ?? 3,
    Animal: ZodiacEnum[c.Animal] ?? 11,
    Birthday: c.Birthday || '',

    // 注数/预算
    GenerateCount: Number(c.Count || 10),
    BudgetYuan: Number(c.BudgetYuan || 0),

    // 过滤/固定
    RedFilter: Array.isArray(c.RedFilter) ? c.RedFilter : [],
    BlueFilter: Array.isArray(c.BlueFilter) ? c.BlueFilter : [],
    FixedRed: Array.isArray(c.FixedRed) ? c.FixedRed : [],

    // 幸运号策略
    FMode: FixedEnum[c.FixedMode] ?? 1,
    FixedPerTicket: Number(c.FixedPerTicket || 0),

    // 覆盖控制
    MaxOverlapRed: Number(c.MaxOverlapRed || 0),
    UsePerNumberCap: !!c.UsePerNumberCap,

    // 锚点配置
    StartBuckets: Array.isArray(c.StartBuckets) ? c.StartBuckets.map(b => ({
      From: Number(b.From||1),
      To:   Number(b.To||28),
      Count:Number(b.Count||0),
    })) : [],
    MaxPerAnchor: Number(c.MaxPerAnchor || 1),

    // 分段模板
    Bands: {
      LowLo: lLo, LowHi: lHi,
      MidLo: mLo, MidHi: mHi,
      HighLo: hLo, HighHi: hHi,
    },
    BandTemplates: Array.isArray(c.BandTemplates)
        ? c.BandTemplates.map(normalizeTpl)  // → [l,m,h]
        : [[2,2,2]],
    TemplateRepeat: Number(c.TemplateRepeat || 2),
  }
}

/* ---------------- 动作 ---------------- */
async function onGenerate(){
  loadingGen.value = true
  try{
    const backendCfg = toBackendConfig(config.value)
    const r = await fetch(`${API}/generate`,{
      method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify({ override:true, config: backendCfg })
    })
    const data = await r.json()
    if(!r.ok) throw new Error(data?.error || '生成失败')
    combos.value = data.combos || []
    stats.value  = data.stats || null
    showToast(`生成成功：${combos.value.length} 注`)
  }catch(e){ showToast(e.message || '生成失败') }
  finally{ loadingGen.value = false }
}

async function fetchLatest(){ try{ const r = await fetch(`${API}/draw/latest`); if(r.ok) latest.value = await r.json() }catch{} }

function pad(n){ return String(n).padStart(2,'0') }

function onPick(e){ const f=e.target.files?.[0]; if(f) uploadFile(f); e.target.value='' }
function onDrop(e){ dragover.value=false; const f=e.dataTransfer.files?.[0]; if(f) uploadFile(f) }
async function uploadFile(file){
  const fd=new FormData(); fd.append('file', file)
  try{
    const r=await fetch(`${API}/history/upload?replace=${replaceImport.value?1:0}`,{method:'POST', body: fd})
    const data=await r.json()
    if(!r.ok) throw new Error(data?.error || '导入失败')
    showToast(`导入成功：${data.imported||0} 行`)
  }catch(e){ showToast(e.message||'导入失败') }
}

/* 复制/导出 */
async function copyCombos() {
  const lines = combos.value.map(c => {
    const reds = (c.reds || []).map(n => String(n).padStart(2,'0')).join(' ');
    const blue = String(c.blue).padStart(2,'0');
    return `${reds} + ${blue}`;
  });
  const text = lines.join('\n') || '';
  try {
    await navigator.clipboard.writeText(text);
    showToast('已复制到剪贴板');
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text; document.body.appendChild(ta);
    ta.select(); document.execCommand('copy'); document.body.removeChild(ta);
    showToast('已复制到剪贴板');
  }
}
function exportCSV() {
  const rows = [['r1','r2','r3','r4','r5','r6','blue']];
  for (const c of combos.value) rows.push([...(c.reds||[]), c.blue]);
  const csv = rows.map(r => r.join(',')).join('\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
  const a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = `ssq_${new Date().toISOString().slice(0,10)}.csv`;
  document.body.appendChild(a); a.click(); document.body.removeChild(a);
  URL.revokeObjectURL(a.href);
}

/* 快捷键 G 触发生成 */
function handleKey(e){
  if (['INPUT','SELECT','TEXTAREA'].includes(e.target?.tagName)) return;
  if (e.key?.toLowerCase() === 'g') onGenerate();
}
onMounted(()=>{ fetchLatest(); window.addEventListener('keydown', handleKey) })
onUnmounted(()=>{ window.removeEventListener('keydown', handleKey) })
</script>

<style scoped>
:global(html){ scrollbar-gutter: stable; }

/* 两列布局：顶部对齐，20px 间隔；统一粘性顶部偏移 */
.page{
  --side: 360px;
  --sticky-top: 16px;
  max-width: 1360px;

  margin: 16px auto; padding: 0 16px;
  display: flex;
  flex-direction: row;
  grid-template-columns: var(--side) 1fr;
  gap: 20px;
  align-items: start;
}
.main{
  width: 966px;
}



/* 工具栏 */
.toolbar{ display:flex; align-items:center; gap:14px; margin-bottom: 12px; }
.latest{ display:flex; align-items:center; gap:8px; color:#374151; }
.badge{ background:#111827; color:#fff; padding:2px 6px; border-radius:6px; font-size:12px; }
.sep{ color:#9ca3af; }
.nums{ display:inline-flex; gap:6px; transform: translateY(1px); }
.ico{ background:transparent; border:0; cursor:pointer; font-size:14px; color:#6b7280; }
.flex-gap{ flex: 1 1 auto; }

.uploader{ display:flex; align-items:center; gap:10px; padding:6px 10px; border:1px dashed #d1d5db; border-radius:10px; }
.uploader.dragover{ background:#f9fafb; }
.ck{ display:flex; align-items:center; gap:6px; color:#4b5563; }

/* 标签栏 */
.tabs{
  display:flex; align-items:center; gap:10px;
  background:#fff; border:1px solid #eee; border-radius:10px; padding:8px;
  position: sticky; top: var(--sticky-top); z-index: 5;
}
.tab{ height:34px; padding:0 14px; border-radius:8px; border:1px solid transparent; background:transparent; cursor:pointer; }
.tab.active{ background:#111827; color:#fff; }
.spacer{ flex:1; }

/* 内容卡片 */
.panel{
  background:#fff; border:1px solid #eee; border-radius:12px; padding:14px; margin-top:12px;
  box-shadow: 0 4px 14px rgba(0,0,0,0.04);
}
.stable{ min-height: 440px; }

.btn{ height:36px; padding:0 12px; border-radius:8px; background:#111827; color:#fff; border:0; cursor:pointer; }
.btn.ghost{ background:#f3f4f6; color:#111; }

.gen-head{ display:flex; align-items:center; }
.hint{ color:#4b5563; }
.ops{ margin-left:auto; display:flex; gap:10px; }

.grid-cards{
  display:grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap:14px; margin-top:12px;
}
@media (min-width: 1200px){ .grid-cards{ grid-template-columns: repeat(4, minmax(0,1fr)); } }
@media (max-width: 1100px){ .grid-cards{ grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 560px){ .grid-cards{ grid-template-columns: 1fr; } }

.card{ border:1px solid #f0f0f0; border-radius:10px; padding:12px; }
.card-h{ display:flex; align-items:center; justify-content:space-between; color:#6b7280; font-size:12px; margin-bottom:8px; }

/* 号码球 */
.nums{ display:flex; flex-wrap:wrap; gap:8px; }
.ball{ display:inline-flex; align-items:center; justify-content:center;
  width:28px; height:28px; border-radius:999px; font-weight:600; font-size:12px;
  background:#f3f4f6; color:#1f2937;
}
.ball.red{ background:#fee2e2; color:#991b1b; }
.ball.blue{ background:#dbeafe; color:#1e3a8a; }

.loading, .empty{ color:#6b7280; padding:12px; }

/* 提示气泡 */
.toast{
  position: fixed; right: 18px; bottom: 18px;
  background:#111827; color:#fff; padding:10px 12px; border-radius:10px; box-shadow: 0 10px 30px rgba(0,0,0,0.15);
}

.config {
  width: 733px;
}
.fade-enter-active,.fade-leave-active{ transition: opacity .2s; }
.fade-enter-from,.fade-leave-to{ opacity: 0; }
</style>
