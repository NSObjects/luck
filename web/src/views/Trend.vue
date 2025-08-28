<template>
  <div class="page">
    <header class="bar">
      <h2>走势 / 热力图</h2>
      <div class="spacer"></div>
      <label class="fld">
        <span>窗口</span>
        <select v-model.number="winSize">
          <option :value="50">近 50 期</option>
          <option :value="100">近 100 期</option>
          <option :value="150">近 150 期</option>
          <option :value="200">近 200 期</option>
        </select>
      </label>
      <button class="btn ghost" @click="load">刷新</button>
    </header>

    <section class="panel">
      <div v-if="!heat" class="loading">加载中…</div>
      <div v-else class="wrap">
        <div class="meta">
          <div>样本期数：<b>{{ heat.BlueVector.length }}</b></div>
          <div class="legend">
            <span class="cell on"></span>出现
            <span class="cell"></span>未出现
          </div>
        </div>

        <div class="heat-grid" :style="{'--cols': Math.max(heat.BlueVector.length, 1)}">
          <div class="row" v-for="n in 33" :key="'r'+n">
            <span class="idx">{{ pad(n) }}</span>
            <span
                v-for="(v, j) in (heat.RedMatrix[n-1] || [])"
                :key="'c'+n+'_'+j"
                class="cell"
                :class="{ on: v===1 }"
                :title="tooltip(n, j)"
            ></span>
          </div>
        </div>

        <details class="blue-line">
          <summary>蓝球序列（从旧到新）</summary>
          <div class="blue-strip">
            <i v-for="(b,i) in heat.BlueVector" :key="'b'+i" class="ball blue" :title="'#'+(i+1)+' → '+pad(b)">{{ pad(b) }}</i>
          </div>
        </details>
      </div>
    </section>

    <section class="panel">
      <div class="grid-2">
        <div class="card">
          <div class="card-h">热点红球（Top 6）</div>
          <ul class="list">
            <li v-for="(p, i) in hot?.TopHotRed || []" :key="'hot'+i">
              <span class="ball red s">{{ pad(p[0]) }}</span>
              <b class="cnt">{{ p[1] }}</b>
            </li>
          </ul>
        </div>
        <div class="card">
          <div class="card-h">冷点红球（Top 6）</div>
          <ul class="list">
            <li v-for="(p, i) in hot?.TopColdRed || []" :key="'cold'+i">
              <span class="ball red s">{{ pad(p[0]) }}</span>
              <b class="cnt">{{ p[1] }}</b>
            </li>
          </ul>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'

const API = import.meta.env.VITE_API_BASE || 'http://localhost:8080/api'

const winSize = ref(100)   // 避免与全局 window 冲突
const heat = ref(null)
const hot  = ref(null)

function pad(n){ return String(n).padStart(2,'0') }

async function load(){
  const [hRes, hoRes] = await Promise.all([
    fetch(`${API}/analysis/heatmap?window=${winSize.value}`),
    fetch(`${API}/analysis/hot?window=${Math.min(winSize.value, 100)}`)
  ])
  heat.value = await hRes.json()
  hot.value  = await hoRes.json()
}

function tooltip(n, j){
  const appear = heat.value?.RedMatrix?.[n-1]?.[j] === 1
  return `期序 #${j+1}：${appear ? '出现' : '未出现'}`
}

watch(winSize, load)
onMounted(load)
</script>

<style scoped>
:global(html){ scrollbar-gutter: stable; }

.page{ max-width: 1200px; margin: 16px auto; padding: 0 12px; display:flex; flex-direction:column; gap:12px; }

.bar{
  display:flex; align-items:center; gap:10px;
  background:#fff; border:1px solid #eee; border-radius:10px; padding:8px 10px;
  position: sticky; top: 16px; z-index: 5;
}
.bar h2{ margin:0; font-size:16px; }
.spacer{ flex:1; }
.fld{ display:flex; align-items:center; gap:6px; }
.fld select{ height:32px; border:1px solid #e5e7eb; border-radius:8px; padding:4px 8px; }
.btn{ height:32px; padding:0 10px; border-radius:8px; background:#111827; color:#fff; border:0; cursor:pointer; }
.btn.ghost{ background:#f3f4f6; color:#111; }

.panel{
  background:#fff; border:1px solid #eee; border-radius:12px; padding:12px;
  box-shadow: 0 4px 14px rgba(0,0,0,0.04);
}
.loading{ color:#6b7280; }

.wrap{ display:flex; flex-direction:column; gap:10px; }
.meta{ display:flex; align-items:center; gap:12px; color:#4b5563; }
.legend{ display:flex; align-items:center; gap:6px; }
.cell{ width:10px; height:10px; border-radius:2px; background:#f3f4f6; display:inline-block; }
.cell.on{ background:#0ea5e9; }

.heat-grid{ --cols: 50; min-width: calc(var(--cols) * 10px + 56px); overflow:auto; }
.row{ display:flex; align-items:center; gap:4px; height: 16px; }
.idx{ display:inline-block; width:40px; color:#6b7280; font-size:12px; text-align:right; padding-right:8px; }

.blue-line summary{ cursor:pointer; color:#374151; }
.blue-strip{ display:flex; flex-wrap:wrap; gap:6px; margin-top:8px; }
.ball{
  display:inline-flex; align-items:center; justify-content:center;
  width:22px; height:22px; border-radius:999px; font-weight:600; font-size:11px;
  background:#f3f4f6; color:#1f2937;
}
.ball.red{ background:#fee2e2; color:#991b1b; }
.ball.blue{ background:#dbeafe; color:#1e3a8a; }
.ball.s{ width:20px; height:20px; }

.grid-2{ display:grid; grid-template-columns: 1fr 1fr; gap:10px; }
.card{ border:1px solid #f1f5f9; border-radius:10px; padding:10px; }
.card-h{ color:#475569; font-size:12px; margin-bottom:6px; }
.list{ list-style:none; padding:0; margin:0; display:flex; flex-wrap:wrap; gap:8px; }
.cnt{ color:#6b7280; margin-left:6px; }
</style>
