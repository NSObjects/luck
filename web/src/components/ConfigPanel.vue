<template>
  <aside class="panel">
    <div class="panel-h">
      <h2>生成配置</h2>
    </div>

    <!-- 基础参数 -->
    <section class="sec">
      <div class="row">
        <label class="fld">
          <span>每次生成注数</span>
          <input type="number" v-model.number="local.Count" min="1" max="200" />
        </label>

        <label class="fld">
          <span>固定号模式</span>
          <select v-model="local.FixedMode">
            <option value="rotate">轮转</option>
            <option value="always">始终包含</option>
          </select>
        </label>

        <label class="fld">
          <span>每注固定号数量</span>
          <input type="number" v-model.number="local.FixedPerTicket" min="0" max="6" />
        </label>

        <label class="fld">
          <span>红球最大重叠</span>
          <input type="number" v-model.number="local.MaxOverlapRed" min="0" max="6" />
        </label>

        <label class="fld">
          <span>策略模式</span>
          <select v-model="local.Mode">
            <option value="mixed">混合</option>
            <option value="random">随机</option>
            <option value="zodiac">生肖</option>
            <option value="birthday">生日</option>
          </select>
        </label>

        <label class="fld">
          <span>生肖</span>
          <select v-model="local.Animal">
            <option v-for="a in animals" :key="a" :value="a">{{ a }}</option>
          </select>
        </label>

        <label class="fld">
          <span>生日</span>
          <input type="date" v-model="local.Birthday" />
        </label>

        <label class="ck fld">
          <span>单号上限</span>
          <div class="h">
            <input type="checkbox" v-model="local.UsePerNumberCap" />
            <i>启用</i>
          </div>
        </label>
      </div>

      <div class="row">
        <label class="fld fld-100">
          <span>红球排除（逗号分隔）</span>
          <input type="text" v-model="redFilterStr" placeholder="如：3,8,11" />
        </label>
        <label class="fld fld-100">
          <span>蓝球排除（逗号分隔）</span>
          <input type="text" v-model="blueFilterStr" placeholder="如：2,6,16" />
        </label>
        <label class="fld fld-100">
          <span>固定红球（逗号分隔）</span>
          <input type="text" v-model="fixedRedStr" placeholder="如：1,12" />
        </label>
      </div>
    </section>

    <!-- 锚点与出现频次限制 -->
    <section class="sec">
      <div class="sec-h"><h3>锚点区间与出现频次</h3></div>

      <div class="row">
        <label class="fld">
          <span>同一锚点最多出现</span>
          <input type="number" v-model.number="local.MaxPerAnchor" min="1" max="6" />
        </label>
        <div class="fld"></div>
      </div>

      <div class="buckets">
        <div class="b-head">
          <span class="c1">From</span>
          <span class="c2">To</span>
          <span class="c3">Count</span>
          <span class="c4"></span>
        </div>
        <div class="b-row" v-for="(b, i) in local.StartBuckets" :key="i">
          <input class="c1" type="number" :min="1" :max="28" v-model.number="b.From" />
          <input class="c2" type="number" :min="1" :max="28" v-model.number="b.To" />
          <input class="c3" type="number" :min="0" :max="local.Count || 200" v-model.number="b.Count" />
          <button class="ico c4" title="删除" @click="removeBucket(i)">✕</button>
        </div>
        <button class="btn ghost small" @click="addBucket">+ 添加区间</button>
        <p class="tip">提示：To 最大建议 28（保证还能取满 6 个数）。各区间的 Count 之和建议 ≥ 生成注数。</p>
      </div>
    </section>

    <!-- 分段区间 & 模板 -->
    <section class="sec">
      <div class="sec-h"><h3>分段区间 & 模板</h3></div>

      <div class="row">
        <label class="fld">
          <span>低段 [lo, hi]</span>
          <div class="pair">
            <input type="number" v-model.number="lowLo" min="1" max="33" />
            <input type="number" v-model.number="lowHi" min="1" max="33" />
          </div>
        </label>
        <label class="fld">
          <span>中段 [lo, hi]</span>
          <div class="pair">
            <input type="number" v-model.number="midLo" min="1" max="33" />
            <input type="number" v-model.number="midHi" min="1" max="33" />
          </div>
        </label>
        <label class="fld">
          <span>高段 [lo, hi]</span>
          <div class="pair">
            <input type="number" v-model.number="highLo" min="1" max="33" />
            <input type="number" v-model.number="highHi" min="1" max="33" />
          </div>
        </label>

        <label class="fld">
          <span>模板轮转次数</span>
          <input type="number" v-model.number="local.TemplateRepeat" min="1" max="10" />
        </label>
      </div>

      <div class="tpls">
        <div class="tpl-row" v-for="(t, i) in local.BandTemplates" :key="i">
          <span class="idx">模板 {{ i + 1 }}</span>
          <input type="number" v-model.number="t.Vals[0]" min="0" max="6" />
          <span class="plus">+</span>
          <input type="number" v-model.number="t.Vals[1]" min="0" max="6" />
          <span class="plus">+</span>
          <input type="number" v-model.number="t.Vals[2]" min="0" max="6" />
          <span class="sum" :class="{ bad: t.Vals[0] + t.Vals[1] + t.Vals[2] !== 6 }">= 6</span>
          <button class="ico" title="删除" @click="removeTpl(i)">✕</button>
        </div>
        <button class="btn ghost small" @click="addTpl">+ 添加模板</button>
      </div>
    </section>

    <div class="panel-actions">
      <button class="btn" @click="$emit('apply', normalized())">应用配置</button>
      <button class="btn ghost" @click="$emit('reset')">重置默认</button>
    </div>
  </aside>
</template>

<script setup>
import { computed, reactive, watch } from 'vue'

const props = defineProps({ modelValue: { type: Object, required: true } })
const emit = defineEmits(['update:modelValue','apply','reset'])

const animals = ['Rat','Ox','Tiger','Rabbit','Dragon','Snake','Horse','Goat','Monkey','Rooster','Dog','Pig']

const local = reactive(JSON.parse(JSON.stringify(props.modelValue)))

// ------ 初始化缺省结构 ------
function ensureBands(){
  local.Bands ||= {}
  local.Bands.Low  ||= [1,11]
  local.Bands.Mid  ||= [12,22]
  local.Bands.High ||= [23,33]
}
function ensureBuckets(){
  if (!Array.isArray(local.StartBuckets)) {
    local.StartBuckets = [
      { From:1, To:10, Count:3 },
      { From:11, To:18, Count:2 },
      { From:19, To:32, Count:2 },
    ]
  }
  if (typeof local.MaxPerAnchor !== 'number') local.MaxPerAnchor = 1
}
ensureBands(); ensureBuckets()

// ------ 绑定区间三段 ------
const lowLo  = computed({ get:()=>local.Bands.Low[0],  set:v=>{ ensureBands(); local.Bands.Low[0]=Number(v)||1 } })
const lowHi  = computed({ get:()=>local.Bands.Low[1],  set:v=>{ ensureBands(); local.Bands.Low[1]=Number(v)||11 } })
const midLo  = computed({ get:()=>local.Bands.Mid[0],  set:v=>{ ensureBands(); local.Bands.Mid[0]=Number(v)||12 } })
const midHi  = computed({ get:()=>local.Bands.Mid[1],  set:v=>{ ensureBands(); local.Bands.Mid[1]=Number(v)||22 } })
const highLo = computed({ get:()=>local.Bands.High[0], set:v=>{ ensureBands(); local.Bands.High[0]=Number(v)||23 } })
const highHi = computed({ get:()=>local.Bands.High[1], set:v=>{ ensureBands(); local.Bands.High[1]=Number(v)||33 } })

// ------ 文本 ↔ 数组 ------
const toStr = (arr)=> (arr||[]).join(',')
const toNums = (s)=> s.split(',').map(x=>x.trim()).filter(Boolean).map(Number).filter(Number.isInteger)
function dedupSort(arr, lo, hi){ return Array.from(new Set(arr.filter(n=>n>=lo && n<=hi))).sort((a,b)=>a-b) }
const redFilterStr = computed({ get:()=>toStr(local.RedFilter), set:v=>local.RedFilter = dedupSort(toNums(v),1,33) })
const blueFilterStr = computed({ get:()=>toStr(local.BlueFilter), set:v=>local.BlueFilter = dedupSort(toNums(v),1,16) })
const fixedRedStr   = computed({ get:()=>toStr(local.FixedRed),   set:v=>local.FixedRed   = dedupSort(toNums(v),1,33) })

// ------ 模板与锚点操作 ------
function addTpl(){ local.BandTemplates ||= []; local.BandTemplates.push({ Vals:[2,2,2] }) }
function removeTpl(i){ local.BandTemplates.splice(i,1) }

function addBucket(){
  ensureBuckets()
  local.StartBuckets.push({
    From: 1, To: 10, Count: 1
  })
}
function removeBucket(i){ local.StartBuckets.splice(i,1) }

// ------ 归一化并同步到父组件 ------
function normalized(){
  const fixRange=(r)=>{
    r[0]=Math.max(1,Math.min(33,Number(r[0]??1)))
    r[1]=Math.max(1,Math.min(33,Number(r[1]??33)))
    if(r[0]>r[1]) [r[0],r[1]]=[r[1],r[0]]
    return r
  }
  ensureBands(); ensureBuckets()
  local.Bands.Low  = fixRange(local.Bands.Low)
  local.Bands.Mid  = fixRange(local.Bands.Mid)
  local.Bands.High = fixRange(local.Bands.High)

  // StartBuckets 规范化
  local.StartBuckets = (local.StartBuckets||[])
      .map(b => {
        let from = Number(b.From||1)
        let to   = Number(b.To||28)
        let cnt  = Number(b.Count||0)
        from = Math.max(1, Math.min(28, from))
        to   = Math.max(1, Math.min(28, to))
        if (from > to) [from, to] = [to, from]
        cnt  = Math.max(0, Math.min(999, cnt))
        return { From: from, To: to, Count: cnt }
      })

  // 模板规范化（和为 6，中段兜底）
  local.BandTemplates = (local.BandTemplates||[]).map(t=>{
    const v=[...(t?.Vals||[0,0,0])].map(x=>Math.max(0,Math.min(6,Number(x||0))))
    const s=v[0]+v[1]+v[2]; if(s!==6) v[1]=Math.max(0,Math.min(6,v[1]+(6-s)))
    return { Vals:v }
  })

  const out = JSON.parse(JSON.stringify(local))
  emit('update:modelValue', out)
  return out
}

// 父变更 → 覆盖
watch(()=>props.modelValue, v=>{
  Object.assign(local, JSON.parse(JSON.stringify(v||{})))
  ensureBands(); ensureBuckets()
}, {deep:true})
</script>

<style scoped>
.panel{
  position: sticky;
  top: var(--sticky-top, 16px);
  z-index: 4;
  width: var(--side, 360px); min-width: var(--side, 360px);
  align-self: start;
  background:#fff; border:1px solid #eee; border-radius:12px;
  box-shadow: 0 4px 14px rgba(0,0,0,0.04);
  padding:16px; max-height: calc(100vh - 16px); overflow:auto;
}
.panel-h{ display:flex; flex-direction:column; gap:4px; margin-bottom:8px; }
.panel-h h2{ font-size:18px; margin:0; }

.sec{ border-top:1px dashed #eee; padding-top:12px; margin-top:12px; }
.sec-h h3{ margin:6px 0 8px; font-size:14px; color:#333; }

.row{ display:grid; grid-template-columns: 1fr 1fr; gap:12px; }
.fld{ display:flex; flex-direction:column; gap:6px; }
.fld-100{ grid-column: 1 / -1; }
.fld input[type="number"], .fld input[type="text"], .fld input[type="date"], .fld select{
  height:36px; padding:6px 10px; border:1px solid #e5e7eb; border-radius:8px; outline:none;
}
.ck .h{ display:flex; align-items:center; gap:8px; color:#4b5563; }
.pair{ display:flex; gap:8px; }

.tpls{ display:flex; flex-direction:column; gap:8px; margin-top:8px; }
.tpl-row{ display:flex; align-items:center; gap:6px; }
.tpl-row input{ width:56px; height:32px; padding:4px 6px; border:1px solid #e5e7eb; border-radius:8px; }
.idx{ width:68px; color:#666; }
.sum{ color:#777; font-size:12px; margin-left:4px; }
.sum.bad{ color:#c81e1e; font-weight:600; }
.plus{ color:#9ca3af; }
.ico{ border:0; background:transparent; cursor:pointer; margin-left:auto; font-size:14px; color:#888; }

.buckets{ display:flex; flex-direction:column; gap:8px; margin-top:8px; }
.b-head, .b-row{
  display:grid; grid-template-columns: 1fr 1fr 1fr 36px; gap:8px; align-items:center;
}
.b-head{ color:#6b7280; font-size:12px; padding:0 2px; }
.b-row input{ height:32px; padding:4px 8px; border:1px solid #e5e7eb; border-radius:8px; }
.tip{ color:#6b7280; font-size:12px; margin:4px 2px 0; }

.panel-actions{ display:flex; gap:10px; margin-top:14px; }
.btn{ height:36px; padding:0 12px; border-radius:8px; background:#111827; color:#fff; border:0; cursor:pointer; }
.btn.ghost{ background:#f3f4f6; color:#111; }
.btn.small{ height:30px; font-size:12px; }
</style>
