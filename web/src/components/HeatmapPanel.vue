<template>
  <section class="panel">
    <div class="head">
      <div class="title">热力图（红 1–33 × 最近 {{ windowSize }} 期）</div>
      <div class="ctrl">
        <label>窗口期：</label>
        <input type="number" v-model.number="windowSize" min="30" max="200" />
        <button class="btn ghost" @click="load">刷新</button>
      </div>
    </div>

    <div v-if="loading" class="loading">加载中...</div>

    <div v-else class="wrap">
      <div class="legend">
        <span class="dot hit"></span>出现
        <span class="dot miss"></span>未出现
      </div>

      <div class="grid" :style="{ gridTemplateColumns: `60px repeat(${cols}, 14px)` }">
        <!-- 表头（从旧到新） -->
        <div class="th">号码 \ 期次</div>
        <div v-for="c in cols" :key="'h'+c" class="th tnum">{{ c }}</div>

        <!-- 33 行红球 -->
        <template v-for="r in 33" :key="'row'+r">
          <div class="rlabel">{{ pad(r) }}</div>
          <div v-for="c in cols" :key="'c'+r+'-'+c"
               class="cell" :class="{ hit: redMatrix[r-1]?.[c-1]===1 }"></div>
        </template>

        <!-- 蓝球一行（可选） -->
        <div class="rlabel blue">蓝球</div>
        <div v-for="c in cols" :key="'b'+c" class="cell blue"
             :class="{ hit: blueVector[c-1] && blueVector[c-1] >= 1 }"></div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'

const props = defineProps({
  api: { type: String, required: true },
  initialWindow: { type: Number, default: 100 }
})

const windowSize = ref(props.initialWindow)
const loading = ref(false)
const redMatrix = ref([])   // 33 x N 0/1
const blueVector = ref([])  // N
const cols = computed(()=> Math.max(0, blueVector.value.length))

function pad(n){ return String(n).padStart(2,'0') }

async function load(){
  loading.value = true
  try{
    const r = await fetch(`${props.api}/analysis/heatmap?window=${windowSize.value}`)
    const d = await r.json()
    redMatrix.value = d.redMatrix || d.RedMatrix || []
    blueVector.value = d.blueVector || d.BlueVector || []
  }finally{
    loading.value = false
  }
}

watch(windowSize, v=>{ if(v<30) windowSize.value=30; if(v>200) windowSize.value=200 })
onMounted(load)
</script>

<style scoped>
.panel{ background:#fff; border:1px solid #eee; border-radius:12px; padding:12px; }
.head{ display:flex; align-items:center; gap:12px; margin-bottom:8px; }
.head .title{ font-weight:600; }
.ctrl{ margin-left:auto; display:flex; align-items:center; gap:8px; }
.ctrl input{ width:80px; height:32px; padding:4px 8px; border:1px solid #e5e7eb; border-radius:8px; }

.wrap{ overflow:auto; }
.legend{ display:flex; align-items:center; gap:12px; color:#6b7280; font-size:12px; margin:6px 2px 8px; }
.dot{ display:inline-block; width:12px; height:12px; border-radius:2px; background:#f3f4f6; border:1px solid #e5e7eb; vertical-align:middle; }
.dot.hit{ background:#fecaca; border-color:#fca5a5; }
.dot.miss{ background:#f3f4f6; border-color:#e5e7eb; }

.grid{
  display:grid;
  gap:2px;
  align-items:center;
  user-select:none;
}
.th{ position:sticky; left:0; background:#fff; z-index:1; font-size:12px; color:#6b7280; padding:2px 4px; }
.tnum{ text-align:center; color:#9ca3af; font-size:10px; }

.rlabel{ position:sticky; left:0; background:#fff; z-index:1; padding:0 4px; font-variant-numeric: tabular-nums; }
.rlabel.blue{ color:#1e3a8a; font-weight:600; }

.cell{
  width:14px; height:14px; border-radius:2px;
  background:#f3f4f6; border:1px solid #e5e7eb;
}
.cell.hit{ background:#ef4444; border-color:#f87171; }
.cell.blue{ background:#e5ecff; border-color:#dbeafe; }
.cell.blue.hit{ background:#3b82f6; border-color:#60a5fa; }
.btn{ height:32px; padding:0 10px; border-radius:8px; background:#111827; color:#fff; border:0; cursor:pointer; }
.btn.ghost{ background:#f3f4f6; color:#111; }
</style>
