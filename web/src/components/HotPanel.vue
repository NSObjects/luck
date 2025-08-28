<template>
  <section class="panel">
    <div class="head">
      <div class="title">热度分析</div>
      <div class="ctrl">
        <label>窗口期：</label>
        <input type="number" v-model.number="windowSize" min="20" max="300" />
        <button class="btn ghost" @click="load">刷新</button>
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>
    <div v-else class="grid">
      <div class="card">
        <div class="card-h">Top 热（红）</div>
        <ol class="list">
          <li v-for="(p,i) in hotTop" :key="'h'+i"><b>{{ pad(p[0]) }}</b><span class="bar" :style="{width: pct(p[1], maxRedFreq)+'%'}"></span><em>{{ p[1] }}</em></li>
        </ol>
      </div>

      <div class="card">
        <div class="card-h">Top 冷（红）</div>
        <ol class="list">
          <li v-for="(p,i) in coldTop" :key="'c'+i"><b>{{ pad(p[0]) }}</b><span class="bar cold" :style="{width: pct(p[1], maxRedFreq)+'%'}"></span><em>{{ p[1] }}</em></li>
        </ol>
      </div>

      <div class="card span2">
        <div class="card-h">红球频次（最近 {{ windowSize }} 期）</div>
        <div class="bars">
          <div class="bar-row" v-for="n in 33" :key="'rf'+n">
            <label>{{ pad(n) }}</label>
            <div class="bar-box"><span class="fill red" :style="{ width: pct(redFreq[n]||0, maxRedFreq)+'%' }"></span></div>
            <em>{{ redFreq[n] || 0 }}</em>
          </div>
        </div>
      </div>

      <div class="card span2">
        <div class="card-h">蓝球频次（最近 {{ windowSize }} 期）</div>
        <div class="bars">
          <div class="bar-row" v-for="n in 16" :key="'bf'+n">
            <label>{{ pad(n) }}</label>
            <div class="bar-box"><span class="fill blue" :style="{ width: pct(blueFreq[n]||0, maxBlueFreq)+'%' }"></span></div>
            <em>{{ blueFreq[n] || 0 }}</em>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'

const props = defineProps({
  api: { type: String, required: true },
  initialWindow: { type: Number, default: 50 }
})

const windowSize = ref(props.initialWindow)
const loading = ref(false)
const data = ref(null)

const redFreq = computed(()=> (data.value?.redFreq)||{})
const blueFreq = computed(()=> (data.value?.blueFreq)||{})
const hotTop = computed(()=> (data.value?.topHotRed)||[])
const coldTop = computed(()=> (data.value?.topColdRed)||[])
const maxRedFreq = computed(()=> Math.max(1, ...Object.values(redFreq.value)))
const maxBlueFreq = computed(()=> Math.max(1, ...Object.values(blueFreq.value)))

function pct(v, max){ return Math.round((v*100)/Math.max(1,max)) }
function pad(n){ return String(n).padStart(2,'0') }

async function load(){
  loading.value = true
  try{
    const r = await fetch(`${props.api}/analysis/hot?window=${windowSize.value}`)
    data.value = await r.json()
  }finally{
    loading.value = false
  }
}

watch(windowSize, (v)=>{ if (v<20) windowSize.value=20; if(v>300) windowSize.value=300 })
onMounted(load)
</script>

<style scoped>
.panel{ background:#fff; border:1px solid #eee; border-radius:12px; padding:12px; }
.head{ display:flex; align-items:center; gap:12px; margin-bottom:8px; }
.head .title{ font-weight:600; }
.ctrl{ margin-left:auto; display:flex; align-items:center; gap:8px; }
.ctrl input{ width:80px; height:32px; padding:4px 8px; border:1px solid #e5e7eb; border-radius:8px; }

.grid{ display:grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap:12px; }
.span2{ grid-column: 1 / -1; }

.card{ border:1px solid #f0f0f0; border-radius:10px; padding:10px; background:#fff; }
.card-h{ font-weight:600; color:#374151; margin-bottom:8px; }

.list{ margin:0; padding:0 4px; list-style:none; display:flex; flex-direction:column; gap:8px; }
.list li{ display:grid; grid-template-columns: 36px 1fr 40px; align-items:center; gap:8px; }
.list b{ text-align:center; background:#f3f4f6; border-radius:8px; padding:4px 0; width:36px; }
.list em{ color:#6b7280; font-style:normal; text-align:right; }
.list .bar{ height:10px; background:#fecaca; border-radius:999px; }
.list .bar.cold{ background:#e5e7eb; }

.bars{ display:grid; grid-template-columns: repeat(3, 1fr); gap:8px 16px; }
@media (max-width: 1080px){ .bars{ grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 720px){ .bars{ grid-template-columns: 1fr; } }

.bar-row{ display:grid; grid-template-columns: 30px 1fr 36px; gap:8px; align-items:center; }
.bar-box{ height:12px; background:#f3f4f6; border-radius:999px; overflow:hidden; }
.fill{ display:block; height:100%; border-radius:999px; }
.fill.red{ background:#ef4444; }
.fill.blue{ background:#3b82f6; }

.loading{ color:#6b7280; padding:10px; }
.btn{ height:32px; padding:0 10px; border-radius:8px; background:#111827; color:#fff; border:0; cursor:pointer; }
.btn.ghost{ background:#f3f4f6; color:#111; }
</style>
