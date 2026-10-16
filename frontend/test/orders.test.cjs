const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const frontend = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(frontend, 'index.html'), 'utf8').match(/<script>([\s\S]*?)<\/script>/)[1];
const user = {id:'9007199254740993',name:'测试用户',role:'user',token:'test-token'};
const row = {id:'9007199254740995',user_id:user.id,order_no:'OD_FROM_API',amount:1299,status:1,expire_time:new Date(Date.now()+600000).toISOString()};
async function setup(account=user,validateSession=true,legacyOrders=false){
  const root={innerHTML:''},requests=[],pending=[],timers=[];
  const storage=new Map([['demo-session-v2',JSON.stringify(account)],['demo-db-v2',JSON.stringify({users:[],products:[],orders:[{userId:user.id,orderNo:'STALE_LOCAL_ORDER'}]})]]);
  const context=vm.createContext({URLSearchParams,Uint8Array,console,window:{DEMO_API_BASE:"http://localhost:8080"},
    location:{search:''},crypto:{randomUUID:()=> 'log-id',getRandomValues:bytes=>bytes.fill(1)},
    localStorage:{getItem:key=>storage.get(key)??null,setItem:(key,value)=>storage.set(key,value),removeItem:key=>storage.delete(key)},
    document:{querySelectorAll:()=>[],querySelector:selector=>selector==='#root'?root:{textContent:'',classList:{add(){},remove(){}}}},setTimeout(callback,delay){timers.push({callback,delay})},setInterval(){},
    fetch:(url,options)=>{requests.push({url,options});if(validateSession&&url.endsWith('/api/auth/session'))return Promise.resolve({status:200,ok:true,json:async()=>({code:0,data:{id:account.id,name:account.name,role:account.role}})});return new Promise(resolve=>pending.push({url,resolve}))}
  });
  vm.runInContext(fs.readFileSync(path.join(frontend,'product_search.js'),'utf8'),context);
  const orderScript=fs.readFileSync(path.join(frontend,'order_search.js'),'utf8');
  vm.runInContext(legacyOrders?orderScript.split('function orderUserDetail(order)')[0]:orderScript,context);
  vm.runInContext(source,context);
  await flush();
  return {root,requests,fireTimers(delay){for(const timer of timers.filter(t=>t.delay===delay)){timers.splice(timers.indexOf(timer),1);timer.callback()}},run:code=>vm.runInContext(code,context),reply(match,data,status=200,envelope=true){const i=pending.findIndex(p=>p.url.includes(match));assert.ok(i>=0,match);if((match==='/api/orders/list'||match==='/api/admin/orders/list')&&Array.isArray(data))data={items:data,total:data.length,page:1,page_size:20};pending.splice(i,1)[0].resolve({status,ok:status<400,json:async()=>!envelope?data:status<400?{code:0,message:"success",data}: {code:50001,...data}})}};
}
const flush=()=>new Promise(resolve=>setImmediate(resolve));
test('订单页发送带身份的 PUT，映射真实字段，重进和刷新重新查询',async()=>{
  const app=await setup();
  // 执行页面实际生成的点击处理器。
  const handler=app.root.innerHTML.match(/onclick="([^"]+)"[^>]*>我的订单/)[1];
  app.run(handler);
  const request=app.requests.find(r=>r.url.includes('/api/orders/list'));
  const query=new URL(request.url).searchParams;assert.equal(query.get('user_id'),user.id);assert.equal(query.get('status'),'1');assert.equal(query.get('sort'),'updated_desc');assert.equal(query.get('page'),'1');
  assert.equal(request.options.method,'PUT');assert.equal(request.options.headers.Authorization,'Bearer test-token');
  assert.match(app.root.innerHTML,/正在加载订单/);assert.doesNotMatch(app.root.innerHTML,/暂无符合条件|STALE_LOCAL_ORDER/);
  app.reply('/api/orders/list',[row]);await flush();
  assert.match(app.root.innerHTML,/OD_FROM_API/);assert.match(app.root.innerHTML,/¥12.99/);assert.match(app.root.innerHTML,/暂无明细/);
  app.run("switchTab('userTab','products');switchTab('userTab','orders')");
  app.reply('/api/orders/list',[]);await flush();assert.match(app.root.innerHTML,/暂无符合条件的待支付订单/);
  app.run('refreshUserOrders()');app.reply('/api/orders/list',{message:'服务暂不可用'},500);await flush();
  assert.match(app.root.innerHTML,/服务暂不可用/);assert.doesNotMatch(app.root.innerHTML,/暂无符合条件|0 笔订单/);
  app.run('refreshUserOrders()');app.reply('/api/orders/list',[row]);await flush();assert.match(app.root.innerHTML,/OD_FROM_API/);
});
test('退出或换账号后，旧请求不能写回订单',async()=>{
  const app=await setup();app.run("switchTab('userTab','orders')");
  app.run("session=null;resetUserOrders();render()");app.reply('/api/orders/list',[row]);await flush();
  assert.equal(app.run('userOrderRows.length'),0);assert.doesNotMatch(app.root.innerHTML,/OD_FROM_API/);
  app.run('saveSession('+JSON.stringify({...user,id:'2',token:'next-token'})+');render()');
  assert.ok(app.requests.some(r=>new URL(r.url).searchParams.get('user_id')==='2'));
});
for(const batch of [false,true])test((batch?'批量':'单个')+'下单成功后查询订单',async()=>{
  const app=await setup();app.reply('/api/products/list',{items:[{id:'7',name:'商品',price:100,stock:5}],total:1});await flush();
  app.run(batch?"selectedProducts.add('7');createSelectedOrder()":"createOrder('7')");
  app.reply('/api/orders',row,201);await flush();
  assert.equal(app.requests.some(r=>r.url.includes('/api/orders/list')),false);
  app.fireTimers(1000);await flush();
  assert.ok(app.requests.some(r=>r.url.includes('/api/orders/list')));
  app.reply('/api/orders/list',[row]);await flush();assert.match(app.root.innerHTML,/OD_FROM_API/);
});

test('状态tab重新请求、筛选金额转分、分页保留条件，旧请求不能覆盖新tab',async()=>{
 const app=await setup();app.run("switchTab('userTab','orders')");
 assert.match(app.root.innerHTML,/role="tab"/);assert.match(app.root.innerHTML,/已支付/);assert.match(app.root.innerHTML,/已完成/);
 app.run('changeOrderStatus(2)');
 let query=new URL(app.requests.at(-1).url).searchParams;assert.equal(query.get('status'),'2');assert.equal(query.get('page'),'1');
 app.reply('/api/orders/list',[row]);await flush();assert.doesNotMatch(app.root.innerHTML,/OD_FROM_API/);
 app.reply('/api/orders/list',[{...row,status:2,order_no:'PAID_ORDER'}]);await flush();assert.match(app.root.innerHTML,/PAID_ORDER/);
 app.run("orderFilters={sort:'price_desc',price_min:'12.99',price_max:'100.00',created_from:'',created_to:''};changeOrderPage(2)");
 query=new URL(app.requests.at(-1).url).searchParams;
 assert.equal(query.get('sort'),'price_desc');assert.equal(query.get('price_min'),'1299');assert.equal(query.get('price_max'),'10000');assert.equal(query.get('page'),'2');assert.equal(query.get('status'),'2');
 app.reply('/api/orders/list',{items:[{...row,status:2}],total:45,page:2,page_size:20});await flush();assert.match(app.root.innerHTML,/共 45 笔 · 第 2 \/ 3 页/);
 app.run('resetOrderFilters()');query=new URL(app.requests.at(-1).url).searchParams;
 assert.equal(query.get('page'),'1');assert.equal(query.get('sort'),'updated_desc');assert.equal(query.has('price_min'),false);
});

test('用户模拟支付和确认收货更新状态，不重复发起操作',async()=>{
 const app=await setup();app.run("switchTab('userTab','orders')");app.reply('/api/orders/list',[row]);await flush();
 assert.match(app.root.innerHTML,/>支付<\/button>/);
 app.run("performOrderAction('"+row.id+"','pay');performOrderAction('"+row.id+"','pay')");
 assert.equal(app.requests.filter(r=>r.url.includes('/pay')).length,1);
 app.reply('/pay',{...row,status:2});await flush();
 assert.equal(app.run('userOrderRows[0].status'),2);assert.doesNotMatch(app.root.innerHTML,/OD_FROM_API/);
 app.run('changeOrderStatus(2)');app.reply('/api/orders/list',[{...row,status:2}]);await flush();assert.match(app.root.innerHTML,/确认收货/);
 app.run("performOrderAction('"+row.id+"','receive')");app.reply('/receive',{...row,status:3});await flush();assert.equal(app.run('userOrderRows[0].status'),3);
});
test('管理员查询全部用户订单，逐条发货不修改其他明细和订单状态',async()=>{
 const admin={...user,id:'1',role:'admin'};
 const app=await setup(admin);app.run("switchTab('adminTab','orders')");
 const request=app.requests.find(r=>r.url.includes('/api/admin/orders/list'));assert.ok(request);assert.equal(new URL(request.url).searchParams.has('user_id'),false);
 app.reply('/api/admin/orders/list',[]);await flush();app.run('changeOrderStatus(2)');
 const paid={...row,status:2,items:[{id:'11',product_id:'7',quantity:1,unit_price_fen:100,status:1},{id:'12',product_id:'8',quantity:1,unit_price_fen:200,status:1}]};
 app.reply('/api/admin/orders/list',[paid]);await flush();
 assert.match(app.root.innerHTML,/用户ID/);assert.match(app.root.innerHTML,/标记已发货/);assert.doesNotMatch(app.root.innerHTML,/STALE_LOCAL_ORDER/);
 app.run("shipOrderItem('"+row.id+"','11')");
 app.reply('/items/11/ship',{...paid.items[0],status:2});await flush();
 assert.equal(app.run('userOrderRows[0].status'),2);assert.equal(app.run('userOrderRows[0].items[0].status'),2);assert.equal(app.run('userOrderRows[0].items[1].status'),1);
 assert.match(app.root.innerHTML,/已发货/);
});

test('统一响应协议拒绝裸数据和 HTTP 200 业务错误',async()=>{
 const app=await setup();
 for(const [body,message] of [[{items:[]},'响应协议无效'],[{code:40901,message:'库存不足'},'库存不足']]){
  const request=app.run("api('/protocol-check')");
  const rejected=assert.rejects(request,new RegExp(message));
  app.reply('/protocol-check',body,200,false);
  await rejected;
 }
 const request=app.run("api('/protocol-check')");
 app.reply('/protocol-check',{id:'9007199254740993'});
 assert.equal((await request).id,'9007199254740993');
});

test('订单接口会话过期时清理本地登录并回到登录页',async()=>{
 const app=await setup();
 app.run("switchTab('userTab','orders')");
 app.reply('/api/orders/list',{message:'登录已过期，请重新登录'},401);
 await flush();
 assert.equal(app.run('session'),null);
 assert.equal(app.run("localStorage.getItem('demo-session-v2')"),null);
 assert.equal(app.run('userOrderRows.length'),0);
 assert.match(app.root.innerHTML,/欢迎回来/);
 assert.doesNotMatch(app.root.innerHTML,/Personal workspace/);
});

test('旧会话的401不能清除新账号会话',async()=>{
 const app=await setup();
 app.run("switchTab('userTab','orders')");
 app.run('saveSession('+JSON.stringify({...user,id:'2',token:'next-token'})+');render()');
 app.reply('/api/orders/list',{message:'登录已过期'},401);
 await flush();
 assert.equal(app.run('session.token'),'next-token');
 assert.equal(JSON.parse(app.run("localStorage.getItem('demo-session-v2')")).id,'2');
 assert.doesNotMatch(app.root.innerHTML,/欢迎回来/);
});

test('服务异常和权限不足不会清理登录，登录失败也不会清理已有会话',async()=>{
 const app=await setup();
 for(const [path,status] of [['/api/orders/list',500],['/api/admin/orders/list',403],['/api/auth/login',401]]){
  const request=app.run('api('+JSON.stringify(path)+')');
  const rejected=assert.rejects(request,/请求失败/);
  app.reply(path,{message:'请求失败'},status);
  await rejected;
  assert.equal(app.run('session.token'),user.token);
 }
});

test('商品请求遇到过期会话也会返回登录页',async()=>{
 const app=await setup();
 app.reply('/api/products/list',{items:[],total:0});await flush();
 assert.equal(app.run('session.token'),user.token);
 app.run('changeProductPage(1)');
 app.reply('/api/products/list',{message:'登录已过期'},401);await flush();
 assert.equal(app.run('session'),null);
});


test('页面启动先校验会话，通过前不展示工作台、不查询商品',async()=>{
 const app=await setup(user,false);
 assert.match(app.root.innerHTML,/正在校验登录状态/);
 assert.equal(app.requests.some(r=>r.url.includes('/api/products/list')),false);
 app.reply('/api/auth/session',{message:'登录已过期'},401);await flush();
 assert.match(app.root.innerHTML,/欢迎回来/);
 assert.equal(app.run('session'),null);
});


test('支付倒计时按后端截止时间推进，到期禁用按钮并拦截支付请求',async()=>{
 const app=await setup();
 app.run('Date.now=()=>100000');
 app.run("switchTab('userTab','orders')");
 app.reply('/api/orders/list',[{...row,expire_time:new Date(161000).toISOString()}]);await flush();
 assert.match(app.root.innerHTML,/剩余 01:01/);
 assert.match(app.root.innerHTML,/>支付<\/button>/);
 app.run(`var countdown={dataset:{orderCountdown:'${row.id}'},textContent:''};
 var payButton={dataset:{orderPay:'${row.id}'},disabled:false,textContent:''};
 document.querySelectorAll=selector=>selector==='[data-order-countdown]'?[countdown]:[payButton];
 Date.now=()=>160001;updateOrderCountdowns();`);
 assert.equal(app.run('countdown.textContent'),'剩余 00:01');
 assert.equal(app.run('payButton.disabled'),false);
 app.run('Date.now=()=>161000;updateOrderCountdowns()');
 assert.equal(app.run('countdown.textContent'),'已超时，等待关闭');
 assert.equal(app.run('payButton.disabled'),true);
 app.run("performOrderAction('"+row.id+"','pay')");
 assert.equal(app.requests.filter(r=>r.url.endsWith('/pay')).length,0);
});

test('缺失或非法截止时间不能支付，已支付订单不显示倒计时',async()=>{
 const app=await setup();
 for(const expire_time of [undefined,'invalid']){
  app.run("switchTab('userTab','orders')");
  app.reply('/api/orders/list',[{...row,expire_time}]);await flush();
  assert.match(app.root.innerHTML,/支付截止时间不可用/);
  assert.match(app.root.innerHTML,/disabled[^>]*>暂不可支付/);
  app.run("performOrderAction('"+row.id+"','pay')");
 }
 assert.equal(app.requests.filter(r=>r.url.endsWith('/pay')).length,0);
 app.run('changeOrderStatus(2)');app.reply('/api/orders/list',[{...row,status:2}]);await flush();
 assert.doesNotMatch(app.root.innerHTML,/data-order-countdown/);
 assert.match(app.root.innerHTML,/确认收货/);
});


test('订单图片和名称链接到订单明细快照，而非当前商品详情',async()=>{
 const app=await setup();
 app.run("switchTab('userTab','orders')");
 app.reply('/api/orders/list',[{...row,items:[{id:'11',product_id:'7',product_name:'旧名称',image:'https://example.com/old.png',unit_price_fen:1100,quantity:1,status:1}]}]);await flush();
 assert.match(app.root.innerHTML,/order_snapshot.html\?order_id=9007199254740995&amp;item_id=11/);
 assert.match(app.root.innerHTML,/旧名称/);
 assert.doesNotMatch(app.root.innerHTML,/product_detail.html/);
});

test('管理员按订单展开用户名字和邮箱，收起后重新读取',async()=>{
 const app=await setup({...user,role:'admin',id:'1'});
 app.run("switchTab('adminTab','orders')");app.reply('/api/admin/orders/list',[row]);await flush();
 assert.match(app.root.innerHTML,/用户详情/);
 app.run("toggleOrderUser('"+row.id+"')");
 assert.match(app.root.innerHTML,/正在加载用户详情/);
 app.reply('/api/admin/orders/'+row.id+'/user',{id:user.id,name:'用户<111>',email:'user111@example.com'});await flush();
 assert.match(app.root.innerHTML,/用户&lt;111&gt;/);assert.match(app.root.innerHTML,/user111@example.com/);
 app.run("toggleOrderUser('"+row.id+"')");assert.doesNotMatch(app.root.innerHTML,/user111@example.com/);
 app.run("toggleOrderUser('"+row.id+"')");assert.equal(app.requests.filter(r=>r.url.endsWith('/user')).length,2);
 app.reply('/api/admin/orders/'+row.id+'/user',{message:'用户不存在或已删除'},404);await flush();assert.match(app.root.innerHTML,/用户不存在或已删除/);
});

test('普通用户不展示用户详情入口，也不能触发详情请求',async()=>{
 const app=await setup();app.run("switchTab('userTab','orders')");app.reply('/api/orders/list',[row]);await flush();
 assert.doesNotMatch(app.root.innerHTML,/用户详情/);
 app.run("toggleOrderUser('"+row.id+"')");assert.equal(app.requests.filter(r=>r.url.endsWith('/user')).length,0);
});

test('退出后不回写仍在加载的用户资料',async()=>{
 const app=await setup({...user,role:'admin',id:'1'});
 app.run("switchTab('adminTab','orders')");app.reply('/api/admin/orders/list',[row]);await flush();
 app.run("toggleOrderUser('"+row.id+"');session=null;resetUserOrders();render()");
 app.reply('/api/admin/orders/'+row.id+'/user',{id:user.id,name:'旧用户',email:'old@example.com'});await flush();
 assert.equal(app.run('orderUserDetails.size'),0);assert.doesNotMatch(app.root.innerHTML,/old@example.com/);
});


test('用户订单切换不依赖管理员详情脚本的初始化',async()=>{
 const app=await setup(user,true,true);
 const click=app.root.innerHTML.match(/onclick="([^"]+)"[^>]*>我的订单/)[1];
 app.run(click);
 assert.match(app.root.innerHTML,/我的订单/);
 assert.ok(app.requests.some(r=>r.url.includes('/api/orders/list')));
 app.reply('/api/orders/list',[]);await flush();
 assert.match(app.root.innerHTML,/暂无符合条件的待支付订单/);
});


test('下单后的延迟跳转不影响退出或更换后的会话',async()=>{
 const app=await setup();
 app.reply('/api/products/list',{items:[{id:'7',name:'商品',price:100,stock:5}],total:1});await flush();
 app.run("createOrder('7')");app.reply('/api/orders',row,201);await flush();
 app.run('session=null;resetUserOrders();render()');app.fireTimers(1000);await flush();
 assert.equal(app.requests.some(r=>r.url.includes('/api/orders/list')),false);
 assert.match(app.root.innerHTML,/欢迎回来/);
});
