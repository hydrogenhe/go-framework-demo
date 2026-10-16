const snapshotContent=document.querySelector('#content');
const snapshotParams=new URLSearchParams(location.search);
function escapeSnapshot(value){return String(value??'').replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[ch]))}
function renderSnapshot(item){
 const image=item.image?`<img src="${escapeSnapshot(item.image)}" alt="${escapeSnapshot(item.product_name)}" onerror="this.replaceWith(document.createTextNode('图片暂不可用'))">`:'暂无商品图片';
 const version=item.product_version>0?escapeSnapshot(item.product_version):'历史订单未记录';
 snapshotContent.className='card';
 snapshotContent.innerHTML=`<div class="visual">${image}</div><section class="details">
 <div class="eyebrow">订单商品快照</div><h1>${escapeSnapshot(item.product_name||'未记录商品名称')}</h1>
 <p class="muted">以下为下单时保存的信息，商品后续修改或下架不影响此快照。</p>
 <div class="price">成交单价 ¥${(item.unit_price_fen/100).toFixed(2)}</div>
 <div class="meta"><span class="label">购买数量</span><span>${escapeSnapshot(item.quantity)}</span></div>
 <div class="meta"><span class="label">明细金额</span><span>¥${(item.unit_price_fen*item.quantity/100).toFixed(2)}</span></div>
 <div class="meta"><span class="label">商品 ID</span><span>${escapeSnapshot(item.product_id)}</span></div>
 <div class="meta"><span class="label">下单商品版本</span><span>${version}</span></div>
 <div class="meta"><span class="label">订单 ID</span><span>${escapeSnapshot(item.order_id)}</span></div>
 </section>`;
}
async function loadSnapshot(){
 let session;
 try{session=JSON.parse(localStorage.getItem('demo-session-v2')||'null')}catch(_){}
 if(!session?.token){location.replace('index.html');return}
 const orderID=snapshotParams.get('order_id'),itemID=snapshotParams.get('item_id');
 if(!/^[1-9]\d*$/.test(orderID||'')||!/^[1-9]\d*$/.test(itemID||'')){snapshotContent.textContent='订单或明细 ID 无效';return}
 try{
  const response=await fetch(`${window.DEMO_API_BASE}/api/orders/${encodeURIComponent(orderID)}/items/${encodeURIComponent(itemID)}/snapshot`,{method:'PUT',headers:{Authorization:'Bearer '+session.token,'X-Log-ID':crypto.randomUUID?.()||`${Date.now()}`}});
  if(response.status===401){
   const current=JSON.parse(localStorage.getItem('demo-session-v2')||'null');
   if(current?.token===session.token)localStorage.removeItem('demo-session-v2');
   location.replace('index.html');return;
  }
  const body=await response.json().catch(()=>null);
  if(!response.ok||body?.code!==0)throw new Error(body?.message||'商品快照加载失败');
  renderSnapshot(body.data);
 }catch(err){snapshotContent.className='state';snapshotContent.textContent=err.message||'商品快照加载失败'}
}
loadSnapshot();
