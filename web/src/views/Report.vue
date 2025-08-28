<template>
  <div class="page">
    <header class="bar">
      <h2>历史报告</h2>
      <div class="spacer"></div>
      <button class="btn ghost" @click="load">刷新</button>
    </header>

    <section class="panel">
      <div v-if="!summary || !hist" class="loading">加载中…</div>
      <div v-else class="grid-3">
        <div class="kpi">
          <span>总记录行数</span>
          <b>{{ hist.total_rows }}</b>
        </div>
        <div class="kpi">
          <span>不同红球组合数</span>
          <b>{{ hist.total_combos }}</b>
        </div>
        <div class="kpi">
          <span>数据库</span>
          <b title="存储文件">{{ hist.store_path }}</b>
        </div>
      </div>
    </section>

    <section class="panel" v-if="hot">
      <div class="card">
        <div class="card-h">红球频次（1-33）</div>
        <div class="bars">
          <div v-for="n in 33" :key="'r'+n" class="bar">
            <span class="idx">{{ pad(n) }}</span>
            <div class="track"><i class="fill" :style="{ width: redPct(n) }"></i></div>
            <span class="val">{{ hot.RedFreq?.[n] || 0 }}</span>
          </div>
        </div>
      </div>
      <div class="card">
        <div class="card-h">蓝球频次（1-16）</div>
        <div class="bars">
          <div v-for="n in 16" :key="'b'+n" class="bar">
            <span class="idx">{{ pad(n) }}</span>
            <div class="track b"><i class="fill b" :style="{ width: bluePct(n) }"></i></div>
            <span class="val">{{ hot.BlueFreq?.[n] || 0 }}</span>
          </div>
        </div>
      </div>
    </section>

    <section class="panel" v-if="summary">
      <div class="grid-2">
        <div class="card">
          <div class="card-h">和值统计（近 50 期区间）</div>
          <div class="kv">
            <div><span>均值</span><b>{{ summary.sumAvg?.toFixed(2) }}</b></div>
            <div><span>范围</span><b>{{ summary.sumMin }} ~ {{ summary.sumMax }}</b></div>
          </div>
        </div>
        <div class="card">
          <div class="card-h">整体分布指标</div>
          <div class="kv">
            <div><span>卡方</span><b>{{ summary.chiSquare?.toFixed(2) }}</b></div>
            <div><span>熵</span><b>{{ summary.entropy?.toFixed(2) }}</b></div>
          </div>
        </div>
      </div>
    </section>

    <section class="panel" v-if="summary">
      <div class="card">
        <div class="card-h">奇偶 / 高低 总计</div>
        <div class="kv">
          <div><span>奇数</span><b>{{ summary.odd }}</b></div>
          <div><span>偶数</span><b>{{ summary.even }}</b></div>
          <div><span>低区(≤16)</span><b>{{ summary.low }}</b></div>
          <div><span>高区(≥17)</span><b>{{ summary.high }}</b></div>
        </div>
      </div>
      <div class="card">
        <div class="card-h">连号长度分布</div>
        <div class="chips">
          <span v-for="(cnt,lenStr) in summary.consecLenDist" :key="'l'+lenStr" class="chip">
            长度 {{ lenStr }}：<b>{{ cnt }}</b>
          </span>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
const API = import.meta.env.VITE_API_BASE || 'http://localhost:8080/api'

const hist = ref(null)     // /history/summary
const hot = ref(null)      // /analysis/hot
const summary = ref(null)  // /analysis/summary

function pad(n){ return String(n).padStart(2,'0') }

async function load(){
  const [h1, h2, h3] = await Promise.all([
    fetch(`${API}/history/summary`),
    fetch(`${API}/analysis/hot?window=200`),
    fetch(`${API}/analysis/summary`),
  ])
  hist.value = await h1.json()
  hot.value = await h2.json()
  summary.value = await h3.json()
}

function redPct(n){
  const v = hot.value?.RedFreq?.[n] || 0
  const max = Math.max(...Array.from({length:33}, (_,i)=>hot.value?.RedFreq?.[i+1]||0), 1)
  return Math.round((v / max) * 100) + '%'
}
function bluePct(n){
  const v = hot.value?.BlueFreq?.[n] || 0
  const max = Math.max(...Array.from({length:16}, (_,i)=>hot.value?.BlueFreq?.[i+1]||0), 1)
  return Math.round((v / max) * 100) + '%'
}

onMounted(load)
</script>

<style scoped>
:global(html){ scrollbar-gutter: stable; }

.page{
  max-width: 1200px; margin: 16px auto; padding: 0 12px; display:flex;
  flex-direction:column; gap:22px; }

.bar{
  display:flex; align-items:center; gap:10px;
  background:#fff; border:1px solid #eee; border-radius:10px; padding:8px 10px;
  position: sticky; top: 16px; z-index: 5;
}
.bar h2{ margin:0; font-size:16px; }
.spacer{ flex:1; }
.btn{ height:32px; padding:0 10px; border-radius:8px; background:#111827; color:#fff; border:0; cursor:pointer; }
.btn.ghost{ background:#f3f4f6; color:#111; }

.panel{
  background:#fff; border:1px solid #eee; border-radius:12px; padding:12px;
  box-shadow: 0 4px 14px rgba(0,0,0,0.04);
}

.grid-3{ display:grid; grid-template-columns: repeat(3, minmax(0,1fr)); gap:10px; }
.kpi{ border:1px solid #f1f5f9; border-radius:10px; padding:10px; display:flex; flex-direction:column; gap:6px; }
.kpi span{ color:#64748b; }
.kpi b{ font-size:18px; }

.card{ border:1px solid #f1f5f9; border-radius:10px; padding:10px; }
.card-h{ color:#475569; font-size:12px; margin-bottom:6px; }

.bars{ display:flex; flex-direction:column; gap:6px; }
.bar{ display:grid; grid-template-columns: 44px 1fr 36px; align-items:center; gap:6px; }
.idx{ color:#6b7280; text-align:right; padding-right:6px; }
.track{ height:10px; background:#f3f4f6; border-radius:999px; overflow:hidden; }
.track.b{ background:#eef2ff; }
.fill{ display:block; height:100%; background:#ef4444; border-radius:999px; }
.fill.b{ background:#3b82f6; }
.val{ color:#374151; text-align:right; }

.grid-2{ display:grid; grid-template-columns: 1fr 1fr; gap:10px; }
.kv{ display:grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap:10px; }
.kv > div{ border:1px solid #f1f5f9; border-radius:10px; padding:10px; display:flex; justify-content:space-between; }
.kv span{ color:#64748b; }

.chips{ display:flex; flex-wrap:wrap; gap:8px; }
.chip{ background:#f3f4f6; border-radius:999px; padding:6px 10px; color:#374151; }
.loading{ color:#6b7280; }
</style>
