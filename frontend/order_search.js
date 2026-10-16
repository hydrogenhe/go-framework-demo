// 三个状态页共用筛选条件，切换状态或筛选后从第一页重新查询。
let orderStatusTab=1,orderPage=1,orderPageSize=20,orderTotal=0;
let orderFilters={sort:'updated_desc',price_min:'',price_max:'',created_from:'',created_to:'',user_id:''};

function buildOrderQuery(userId){
 const params=new URLSearchParams({status:String(orderStatusTab),page:String(orderPage),page_size:String(orderPageSize)});
 if(orderFilters.sort!==undefined&&orderFilters.sort!==null&&String(orderFilters.sort).trim()!=='')params.set('sort',String(orderFilters.sort).trim());
 if(userId!==undefined&&userId!==null&&String(userId)!==''){if(!/^[1-9]\d*$/.test(String(userId)))throw new Error('用户ID必须为正整数');params.set('user_id',String(userId))}
 for(const key of ['price_min','price_max']){
  if(orderFilters[key]!=='')params.set(key,String(filterPriceFen(orderFilters[key])));
 }
 if(params.has('price_min')&&params.has('price_max')&&Number(params.get('price_min'))>Number(params.get('price_max')))throw new Error('最低金额不能大于最高金额');
 for(const key of ['created_from','created_to']){
  if(orderFilters[key])params.set(key,new Date(orderFilters[key]).toISOString());
 }
 if(orderFilters.created_from&&orderFilters.created_to&&new Date(orderFilters.created_from)>new Date(orderFilters.created_to))throw new Error('开始时间不能晚于结束时间');
 return params;
}

function orderSearchForm(){
 return `<form onsubmit="applyOrderFilters(event)" class="form-grid product-search">
 ${session?.role==='admin'?`<div class="field"><label for="order-user-id">用户ID</label><input id="order-user-id" name="user_id" value="${esc(orderFilters.user_id)}" placeholder="全部用户" inputmode="numeric"></div>`:''}
 <div class="field"><label for="order-sort">排序</label><select id="order-sort" name="sort">
 ${[['updated_desc','更新时间倒序'],['updated_asc','更新时间正序'],['price_asc','金额从低到高'],['price_desc','金额从高到低']].map(([value,label])=>`<option value="${value}" ${orderFilters.sort===value?'selected':''}>${label}</option>`).join('')}</select></div>
 <div class="field"><label for="order-price-min">最低金额（元）</label><input id="order-price-min" name="price_min" type="number" min="0" step="0.01" value="${esc(orderFilters.price_min)}" placeholder="不限"></div>
 <div class="field"><label for="order-price-max">最高金额（元）</label><input id="order-price-max" name="price_max" type="number" min="0" step="0.01" value="${esc(orderFilters.price_max)}" placeholder="不限"></div>
 <div class="field"><label for="order-created-from">创建时间从</label><input id="order-created-from" name="created_from" type="datetime-local" value="${esc(orderFilters.created_from)}"></div>
 <div class="field"><label for="order-created-to">创建时间至</label><input id="order-created-to" name="created_to" type="datetime-local" value="${esc(orderFilters.created_to)}"></div>
 <div class="wide" style="display:flex;gap:12px"><button class="primary" style="width:auto;padding:10px 20px" ${loadingOrders?'disabled':''}>查询</button><button type="button" class="secondary" onclick="resetOrderFilters()">重置</button></div>
 </form>`;
}

function orderStatusTabs(){
 return `<div class="tabs order-status-tabs" role="tablist" aria-label="订单状态">${[[1,'待支付'],[2,'已支付'],[3,'已完成'],[4,'已取消']].map(([value,label])=>`<button id="order-status-${value}" type="button" role="tab" aria-selected="${orderStatusTab===value}" aria-controls="order-results" class="${orderStatusTab===value?'active':''}" onclick="changeOrderStatus(${value})">${label}</button>`).join('')}</div>`;
}

function orderPagination(){
 const pages=Math.max(1,Math.min(Math.ceil(orderTotal/orderPageSize),Math.floor(10000/orderPageSize)));
 return `<div class="pagination"><span class="muted">共 ${orderTotal} 笔 · 第 ${orderPage} / ${pages} 页</span><label>每页 <select onchange="changeOrderPage(1,Number(this.value))" ${loadingOrders?'disabled':''}>${[10,20,50,100].map(n=>`<option value="${n}" ${n===orderPageSize?'selected':''}>${n} 笔</option>`).join('')}</select></label><button class="secondary" ${loadingOrders||orderPage<=1?'disabled':''} onclick="changeOrderPage(${orderPage-1})">上一页</button><button class="secondary" ${loadingOrders||orderPage>=pages?'disabled':''} onclick="changeOrderPage(${orderPage+1})">下一页</button></div>`;
}

function changeOrderStatus(status){orderStatusTab=status;orderPage=1;return refreshUserOrders()}
function changeOrderPage(page,size=orderPageSize){orderPage=page;orderPageSize=size;return refreshUserOrders()}
function applyOrderFilters(event){
 event.preventDefault();
 const previous=orderFilters;
 const form=new FormData(event.target);
 orderFilters=Object.fromEntries(['sort','price_min','price_max','created_from','created_to','user_id'].map(key=>[key,String(form.get(key)||'').trim()]));
 try{buildOrderQuery(session.role==='admin'?orderFilters.user_id:session.id)}catch(err){orderFilters=previous;toast(err.message);return}
 orderPage=1;return refreshUserOrders();
}
function resetOrderFilters(){orderFilters={sort:'updated_desc',price_min:'',price_max:'',created_from:'',created_to:'',user_id:''};orderPage=1;return refreshUserOrders()}
function orderCreatedTime(order){return order.create_time?new Date(order.create_time).toLocaleString('zh-CN',{hour12:false}):'—'}
function orderRemainingMilliseconds(order){
 const deadline=Date.parse(order.expire_time);
 return Number.isFinite(deadline)?Math.max(0,deadline-Date.now()):null;
}
function orderCountdownText(order){
 const remaining=orderRemainingMilliseconds(order);
 if(remaining===null)return '支付截止时间不可用';
 if(remaining===0)return '已超时，等待关闭';
 const seconds=Math.ceil(remaining/1000);
 return '剩余 '+String(Math.floor(seconds/60)).padStart(2,'0')+':'+String(seconds%60).padStart(2,'0');
}
function orderExpireTime(order){
 const deadline=Date.parse(order.expire_time);
 const time=Number.isFinite(deadline)?new Date(deadline).toLocaleString('zh-CN',{hour12:false}):'—';
 return time+(order.status===1?`<div class="muted" data-order-countdown="${esc(order.id)}">${orderCountdownText(order)}</div>`:'');
}
// 只更新倒计时与支付按钮，避免每秒重绘表单导致输入丢失。
function updateOrderCountdowns(){
 for(const element of document.querySelectorAll('[data-order-countdown]')){
  const order=userOrderRows.find(order=>order.id===element.dataset.orderCountdown);
  if(order)element.textContent=orderCountdownText(order);
 }
 for(const button of document.querySelectorAll('[data-order-pay]')){
  const order=userOrderRows.find(order=>order.id===button.dataset.orderPay);
  if(!order)continue;
  const remaining=orderRemainingMilliseconds(order);
  button.disabled=orderActionBusy.has(order.id)||order.status!==1||remaining===null||remaining===0;
  button.textContent=orderActionBusy.has(order.id)?'处理中…':remaining===0?'已超时':remaining===null?'暂不可支付':'支付';
 }
}
setInterval(updateOrderCountdowns,1000);

const orderActionBusy=new Set();

function orderActions(order){
 if(session.role==='admin'||order.status===3||order.status===4)return '';
 const action=order.status===1?'pay':'receive';
 const remaining=orderRemainingMilliseconds(order);
 const blocked=order.status===1&&(remaining===null||remaining===0);
 const label=order.status===1?(remaining===0?'已超时':remaining===null?'暂不可支付':'支付'):'确认收货';
 return `<button class="secondary" ${action==='pay'?`data-order-pay="${esc(order.id)}"`:''} ${orderActionBusy.has(order.id)||blocked?'disabled':''} onclick="performOrderAction('${order.id}','${action}')">${orderActionBusy.has(order.id)?'处理中…':label}</button>`;
}

async function performOrderAction(id,action){
 if(orderActionBusy.has(id)||!['pay','receive'].includes(action))return;
 const order=userOrderRows.find(order=>order.id===id);
 if(action==='pay'&&(!order||order.status!==1||!(orderRemainingMilliseconds(order)>0))){toast('订单已超时或支付截止时间不可用，请刷新订单');updateOrderCountdowns();return}
 const token=session?.token;
 orderActionBusy.add(id);render();
 try{
  const updated=await api(`/api/orders/${encodeURIComponent(id)}/${action}`);
  if(session?.token!==token)return;
  const order=userOrderRows.find(order=>order.id===id);
  if(order){const previous=order.status;order.status=updated.status;if(previous===orderStatusTab&&updated.status!==orderStatusTab)orderTotal=Math.max(0,orderTotal-1)}
  toast(action==='pay'?'模拟支付成功':'已确认收货，订单已完成');
 }catch(err){if(session?.token===token)toast(err.message)}
 finally{orderActionBusy.delete(id);if(session?.token===token)render()}
}

function orderSnapshotHref(orderID,itemID){return 'order_snapshot.html?order_id='+encodeURIComponent(orderID)+'&item_id='+encodeURIComponent(itemID)}
function orderItemDetails(order){
 if(!order.items?.length)return '<span class="muted">暂无明细</span>';
 return `<div class="order-items">${order.items.map(item=>`<div class="order-item"><div style="display:flex;align-items:center;gap:12px">${item.image?`<a href="${esc(orderSnapshotHref(order.id,item.id))}" aria-label="查看商品快照"><img src="${esc(item.image)}" alt="${esc(item.product_name||'商品图片')}" style="width:56px;height:56px;object-fit:cover;border-radius:8px" loading="lazy"></a>`:''}<div><a href="${esc(orderSnapshotHref(order.id,item.id))}" style="color:inherit">${esc(item.product_name||'商品快照')} · 查看快照</a><div class="muted">${item.quantity} 件 · 单价 ${money(item.unit_price_fen)}</div></div></div><span class="status ${item.status===2?'ok':''}">${item.status===2?'已发货':'待发货'}</span>${session.role==='admin'&&order.status===2&&item.status!==2?`<button class="secondary" ${orderActionBusy.has(order.id)?'disabled':''} onclick="shipOrderItem('${order.id}','${esc(item.id)}')">${orderActionBusy.has(order.id)?'处理中…':'标记已发货'}</button>`:''}</div>`).join('')}</div>`;
}

async function shipOrderItem(orderId,itemId){
 if(orderActionBusy.has(orderId)||session?.role!=='admin')return;
 const token=session.token;
 orderActionBusy.add(orderId);render();
 try{
  const updated=await api(`/api/orders/${encodeURIComponent(orderId)}/items/${encodeURIComponent(itemId)}/ship`);
  if(session?.token!==token)return;
  const order=userOrderRows.find(order=>order.id===orderId);
  const item=order?.items?.find(item=>String(item.id)===String(itemId));
  if(item)item.status=updated.status;
  toast('该订单明细已标记发货');
 }catch(err){if(session?.token===token)toast(err.message)}
 finally{orderActionBusy.delete(orderId);if(session?.token===token)render()}
}


function orderUserDetail(order){
 const entry=orderUserDetails.get(order.id);
 const detail=!entry?'':entry.loading?'<div class="muted">正在加载用户详情…</div>':entry.error?`<div class="error">${esc(entry.error)}</div>`:`<div style="margin-top:8px"><div>名字：${esc(entry.data.name)}</div><div>邮箱：${esc(entry.data.email)}</div></div>`;
 return `${esc(order.userId)}<div><button class="secondary" onclick="toggleOrderUser('${order.id}')">${entry?'收起用户详情':'用户详情'}</button></div>${detail}`;
}
async function toggleOrderUser(orderID){
 if(session?.role!=='admin')return;
 if(orderUserDetails.has(orderID)){orderUserDetails.delete(orderID);render();return}
 const token=session.token,entry={loading:true};
 orderUserDetails.set(orderID,entry);render();
 try{
  const detail=await api('/api/admin/orders/'+encodeURIComponent(orderID)+'/user');
  if(session?.token!==token||orderUserDetails.get(orderID)!==entry)return;
  entry.data=detail;
 }catch(err){
  if(session?.token!==token||orderUserDetails.get(orderID)!==entry)return;
  entry.error=err.message||'用户详情加载失败';
 }finally{
  if(session?.token===token&&orderUserDetails.get(orderID)===entry){entry.loading=false;render()}
 }
}
