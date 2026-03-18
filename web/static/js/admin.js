const VERSION = '2.9.20'; // Using local version
const layuicss = '/static/lib/layui/css/layui.css';
const layuijs = '/static/lib/layui/layui.js';
const rootPath = (function (src) {
  src = (document.currentScript && document.currentScript.tagName.toUpperCase() === 'SCRIPT') ? document.currentScript.src : document.scripts[document.scripts.length - 1].src;
  return src.substring(0, src.lastIndexOf('/') + 1);
})();

// CSRF令牌管理
const CSRFManager = {
  // 缓存的CSRF令牌
  token: null,
  
  // 获取CSRF令牌
  async getToken() {
    if (this.token) {
      return this.token;
    }
    
    try {
      const response = await fetch('/admin/api/csrf-token', {
        method: 'GET',
        headers: {
          'X-Requested-With': 'XMLHttpRequest'
        }
      });
      
      if (response.ok) {
        const data = await response.json();
        if (data.code === 0 && data.data && data.data.csrf_token) {
          this.token = data.data.csrf_token;
          return this.token;
        }
      }
    } catch (error) {
      console.error('获取CSRF令牌失败:', error);
    }
    
    return null;
  },
  
  // 清除缓存的令牌
  clearToken() {
    this.token = null;
  },
  
  // 为fetch请求添加CSRF令牌
  async addCSRFHeader(headers = {}) {
    const token = await this.getToken();
    if (token) {
      headers['X-CSRF-Token'] = token;
    }
    return headers;
  }
};

// 增强的fetch函数，自动添加CSRF令牌
window.fetchWithCSRF = async function(url, options = {}) {
  const headers = await CSRFManager.addCSRFHeader(options.headers || {});
  return fetch(url, {
    ...options,
    headers
  });
};

const app = document.querySelector('#app')

addLink({ href: layuicss }).then(() => {
  app.style.display = 'block';
});

addLink({ id: 'layui_theme_css', href: `/static/src/layui-theme-dark-selector.css` });

loadScript(layuijs, function () {
  layui
    .config({
      base: '/static/lib/',
    })
    .extend({
      drawer: 'drawer/drawer',
    });
  layui.use(['drawer', 'colorMode', 'jquery', 'layer'], async function () {
    const { $, element, form, layer, util, dropdown, drawer, colorMode } = layui;

    // --- CSRF Setup for jQuery ---
    // Ensure token is loaded
    await CSRFManager.getToken();
    
    $.ajaxSetup({
        beforeSend: function(xhr) {
            if (CSRFManager.token) {
                xhr.setRequestHeader('X-CSRF-Token', CSRFManager.token);
            }
        },
        complete: function(xhr) {
             if (xhr.status === 401) {
                 window.location.href = '/admin/login';
             }
        }
    });
    // -----------------------------

    const APPERANCE_KEY = 'layui-theme-demo-prefer-dark';

    const theme = colorMode.init({
      selector: 'html',
      attribute: 'class',
      initialValue: 'dark',
      modes: {
        light: '',
        dark: 'dark',
      },
      storageKey: APPERANCE_KEY,
      onChanged(mode, defaultHandler) {
        const isAppearanceTransition = document.startViewTransition && !window.matchMedia(`(prefers-reduced-motion: reduce)`).matches;
        const isDark = mode === 'dark';

        $('#change-theme').attr('class', `layui-icon layui-icon-${isDark ? 'moon' : 'light'}`);

        if (!isAppearanceTransition) {
          defaultHandler();
        } else {
          rippleViewTransition(isDark, function () {
            defaultHandler();
          });
        }
      },
    });

    routerTo({path: location.hash.slice(1) || 'dashboard'});

    dropdown.render({
      elem: '#change-theme',
      align: 'center',
      data: [
        {
          title: '深色模式',
          id: 'dark',
          icon: 'layui-icon-moon',
        },
        {
          title: '浅色模式',
          id: 'light',
          icon: 'layui-icon-light',
        },
        {
          title: '跟随系统',
          id: 'auto',
          icon: 'layui-icon-console',
        },
      ],
      templet(d) {
        return `
                <span style="display: flex;">
                  <i class="layui-icon ${d.icon}" style="margin-right: 8px"></i>
                  ${d.title}
                </span>`.trim();
      },
      click(obj) {
        const { id: mode } = obj;
        theme.setMode(mode);
      },
    });

    util.event('lay-header-event', {
      menuLeft() {
        $('body').toggleClass('collapse');
      },
      menuRight() {
        drawer.open({
          area: '600px',
          url: './static/tpl/theme.html',
          hideOnClose: true,
          id: 'drawer-theme-tpl',
          shade: 0.01,
        });
      },
    });

    element.on('nav(nav-side)', function (elem) {
      var path = elem.data('path');
      if (path) {
        routerTo({path});
        if ($(window).width() <= 768) {
          $('body').toggleClass('collapse', false);
        }
      }
    });

    $('#layuiv').text(layui.v);

    /*
     * 后台通用脚本
     * 说明：统一处理全局的退出登录逻辑
     */
    
    // 绑定退出登录按钮事件
    const bindLogout = () => {
      const btn = document.getElementById('logout-btn');
      if (!btn) return;
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        handleLogout();
      });
    };
    
    // 执行退出登录
    const handleLogout = () => {
      layer.confirm('确定要退出登录吗？', {
        icon: 3,
        title: '提示'
      }, (index) => {
        layer.close(index);
        
        const loadIndex = layer.load(2, {
          content: '正在退出登录...'
        });
        
        fetchWithCSRF('/admin/logout', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest'
          }
        })
        .then(response => response.json())
        .then(data => {
          layer.close(loadIndex);
          const ok = data && (data.code === 0 || data.success);
          const msg = (data && (data.msg || data.message)) || (ok ? '退出登录成功' : '退出登录失败');
          if (ok) {
            layer.msg(msg, {
              icon: 1,
              time: 1000
            }, () => {
              const redirect = (data && data.data && data.data.redirect) || '/admin/login';
              window.location.href = redirect;
            });
          } else {
            layer.msg(msg, { icon: 2 });
          }
        })
        .catch(error => {
          layer.close(loadIndex);
          console.error('登出请求失败:', error);
          layer.msg('网络错误，请重试', { icon: 2 });
        });
      });
    };
    
    (() => {
      if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', bindLogout);
      } else {
        bindLogout();
      }
    })();

    // 刷新页面功能处理
    const handleRefresh = () => {
      layer.confirm('确定要刷新内容吗？', {
        icon: 3,
        title: '提示'
      }, (index) => {
        layer.close(index);
        
        let currentPath = window.location.hash.replace('#', '') || 'dashboard';
        
        const loadIndex = layer.load(2, {
          content: '正在刷新...'
        });
        
        setTimeout(() => {
          routerTo({ path: currentPath });
          layer.close(loadIndex);
        }, 500);
      });
    };

    $('#refresh-btn').on('click', handleRefresh);

    // 统一的Tips提示功能
    $(document).off('click', '[data-tips]').on('click', '[data-tips]', function() {
      var tipsType = $(this).data('tips');
      var tipsContent = getTipsContent(tipsType);
      layer.tips(tipsContent, this, {
        tips: [2, '#16b777'],
        time: 3000
      });
    });

    function getTipsContent(type) {
      var tips = {
        'user-username': '用户名：用于登录的用户名，可以修改但需要保证唯一性',
        'user-old-password': '旧密码：修改密码时需要输入当前密码进行验证，不修改密码时可留空',
        'user-new-password': '新密码：要设置的新密码，长度至少6位，不修改密码时可留空',
        'site-title': '站点标题：网站的主标题，显示在浏览器标题栏和搜索引擎结果中',
        'site-keywords': '关键词：网站的SEO关键词，用于搜索引擎优化，多个关键词用逗号分隔',
        'site-description': '站点描述：网站的简要描述，用于SEO和搜索引擎结果展示',
        'site-logo': '站点Logo：网站的标志图片路径，建议使用SVG格式',
        'maintenance-mode': '维护模式：开启后网站将进入维护模式，普通用户无法访问',
        'default-user-role': '默认角色：新注册用户的默认权限级别，0为管理员，1为普通成员',
        'session-timeout': '会话超时：用户登录会话的有效时间，单位为秒，超时后需要重新登录',
        'footer-text': '页脚文本：显示在网站底部的版权信息或其他文本',
        'icp-record': 'ICP备案：网站的ICP备案号，中国大陆网站必须显示',
        'icp-record-link': 'ICP备案链接：ICP备案号对应的查询链接，通常指向工信部备案网站',
        'psb-record': '公安备案：网站的公安备案号，部分地区要求显示',
        'psb-record-link': '公安备案链接：公安备案号对应的查询链接，通常指向公安部备案网站',
        'app-name': '应用名称：设置应用的显示名称，用户在客户端看到的应用标识',
        'app-version': '应用版本：当前应用的版本号，用于版本控制和更新检测',
        'app-status': '应用状态：控制应用是否可用，禁用后用户无法使用该应用',
        'force-update': '强制更新：开启后用户必须更新到最新版本才能使用',
        'download-type': '更新方式：设置应用的更新下载方式，支持不同的分发渠道',
        'download-url': '下载地址：应用安装包的下载链接地址',
        'login-type': '登录方式：设置用户登录验证的方式，如账号密码、卡密等',
        'multi-open-scope': '多开范围：设置多开功能的作用范围，如全局或特定应用',
        'clean-interval': '清理间隔：系统自动清理无效会话的时间间隔（分钟）',
        'check-interval': '校验间隔：系统检查用户状态的时间间隔（分钟）',
        'multi-open-count': '多开数量：允许用户同时运行的应用实例数量',
        'machine-verify': '机器码验证：控制是否启用机器码验证功能，用于限制软件在特定设备上运行',
        'machine-rebind': '机器码重绑：允许用户重新绑定机器码，当设备更换或重装系统时使用',
        'machine-rebind-limit': '重绑限制：设置重绑的时间限制，每天表示每天可重绑，永久表示不限制重绑时间',
        'machine-free-count': '免费次数：用户可以免费重绑机器码的次数',
        'machine-rebind-count': '重绑次数：用户总共可以重绑机器码的次数限制',
        'machine-rebind-deduct': '重绑扣除：每次重绑机器码时扣除的时间（分钟）',
        'ip-verify': 'IP地址验证：控制是否启用IP地址验证，关闭/开启/开启(市)/开启(省)分别对应不同的验证级别',
        'ip-rebind': 'IP地址重绑：允许用户重新绑定IP地址，当网络环境变化时使用',
        'ip-rebind-limit': '重绑限制：设置IP重绑的时间限制，每天表示每天可重绑，永久表示不限制重绑时间',
        'ip-free-count': '免费次数：用户可以免费重绑IP地址的次数',
        'ip-rebind-count': '重绑次数：用户总共可以重绑IP地址的次数限制',
        'ip-rebind-deduct': '重绑扣除：每次重绑IP地址时扣除的时间（分钟）',
        'register-enabled': '账号注册：控制是否允许新用户注册账号',
        'register-limit': '注册限制：设置注册的限制规则，如时间限制等',
        'register-limit-time': '限制时间：注册限制的时间周期，每天或永久',
        'register-count': '注册次数：在限制时间内允许注册的账号数量',
        'trial-enabled': '领取试用：控制是否允许用户领取试用时间',
        'trial-limit-time': '限制时间：试用领取的时间限制周期',
        'trial-time': '试用时间：用户可以领取的试用时长（分钟）',
        'submit-algorithm': '提交算法：客户端向服务器提交数据时使用的加密算法<br/>• 不加密：数据明文传输，适用于内网环境<br/>• RC4：对称加密，速度快，适用于一般场景<br/>• RSA：非对称加密，安全性高，适用于敏感数据<br/>• RSA（动态）：动态生成密钥的RSA加密，安全性最高<br/>• 易加密：自定义对称加密算法，使用15-30位整数密钥数组',
        'submit-keys': '提交密钥：用于加密客户端提交数据的密钥<br/>• RC4：16位十六进制密钥，用于对称加密<br/>• RSA：公钥用于客户端加密，私钥用于服务器解密<br/>• 易加密：15-30位整数数组，逗号分隔<br/>• 密钥由系统自动生成，确保安全性',
        'return-algorithm': '返回算法：服务器向客户端返回数据时使用的加密算法<br/>• 不加密：数据明文传输，适用于内网环境<br/>• RC4：对称加密，速度快，适用于一般场景<br/>• RSA：非对称加密，安全性高，适用于敏感数据<br/>• RSA（动态）：动态生成密钥的RSA加密，安全性最高<br/>• 易加密：自定义对称加密算法，使用15-30位整数密钥数组',
        'return-keys': '返回密钥：用于加密服务器返回数据的密钥<br/>• RC4：16位十六进制密钥，用于对称加密<br/>• RSA：公钥用于服务器加密，私钥用于客户端解密<br/>• 易加密：15-30位整数数组，逗号分隔<br/>• 密钥由系统自动生成，确保安全性',
        'api-status': '接口状态：控制当前API接口是否可用<br/>• 启用：接口正常工作，客户端可以调用<br/>• 禁用：接口暂停服务，客户端调用将返回错误',
        'variable-alias': '变量别名：变量的唯一标识符，必须以英文字母开头，只能包含数字和英文字母，用于在代码中引用该变量',
        'variable-app': '关联应用：选择变量所属的应用，选择"全局变量"表示该变量可在所有应用中使用',
        'variable-data': '变量数据：存储的具体数据内容，可以是文本、数字、JSON等格式，根据实际需要填写',
        'variable-remark': '备注：对该变量的说明和描述，帮助理解变量的用途和使用场景，可选填写',
        'function-alias': '函数别名：函数的唯一标识符，必须以英文字母开头，只能包含数字和英文字母，用于在代码中调用该函数',
        'function-app': '关联应用：选择函数所属的应用，选择"全局函数"表示该函数可在所有应用中使用',
        'function-code': '函数代码：存储的JavaScript代码内容，使用Goja引擎执行，支持ES5语法和部分ES6特性',
        'function-remark': '备注：对该函数的说明和描述，帮助理解函数的功能和使用场景，可选填写'
      };
      return tips[type] || '暂无说明';
    }

    function routerTo({
      elem = '#router-view',
      path = 'dashboard',
      prefix = '/admin/', //路由前缀
      suffix = '', //路由后缀
    } = {}) {
      var routerView = $(elem);
      var url = prefix + path + suffix;

      var loadTimer = setTimeout(() => {
        layer.load(2);
      }, 100);

      history.replaceState({}, '', `#${path}`); 
      routerView.attr('src', url)
      routerView.off('load').on('load',function(){
        element.render();
        form.render();
        clearTimeout(loadTimer);
        layer.closeLast('loading');
      })

      // 选中, 展开菜单
      $('#ws-nav-side')
        .find("[data-path='" + path + "']")
        .parent('dd')
        .addClass('layui-this')
        .closest('.layui-nav-item')
        .addClass('layui-nav-itemed');
    }

  });
});

function rippleViewTransition(isDark, callback) {
  // 移植自 https://github.com/vuejs/vitepress/pull/2347
  // 支持 Chrome 111+

  // 兼容 jQuery 3 下隐式 event 全局对象不可用的问题
  if (!window.event) {
    window.event = new MouseEvent('click', {
      clientX: document.documentElement.clientWidth,
      clientY: 60,
    });
  }

  const x = event.clientX;
  const y = event.clientY;
  const endRadius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y));
  const transition = document.startViewTransition(function () {
    callback && callback();
  });
  transition.ready.then(function () {
    var clipPath = [`circle(0px at ${x}px ${y}px)`, `circle(${endRadius}px at ${x}px ${y}px)`];
    document.documentElement.animate(
      {
        clipPath: isDark ? clipPath : [...clipPath].reverse(),
      },
      {
        duration: 300,
        easing: 'ease-in',
        pseudoElement: isDark ? '::view-transition-new(root)' : '::view-transition-old(root)',
      }
    );
  });
}

function addStyle(id, cssStr) {
  const el = document.getElementById(id) || document.createElement('style');
  if (!el.isConnected) {
    el.type = 'text/css';
    el.id = id;
    document.head.appendChild(el);
  }
  el.textContent = cssStr;
}

function addLink(opt) {
  return new Promise((resolve) => {
    const link = Object.assign(document.createElement('link'), {
      rel: 'stylesheet',
      onload: () => resolve({ ...opt, status: 'success' }),
      onerror: () => resolve({ ...opt, status: 'error' }), // 为了在 Promise.all 的使用场景
      ...opt,
    });
    document.head.appendChild(link);
  });
}

function loadScript(url, callback) {
  const script = document.createElement('script');
  script.type = 'text/javascript';
  script.async = 'async';
  script.src = url;
  document.body.appendChild(script);
  if (script.readyState) {
    script.onreadystatechange = function () {
      if (script.readyState == 'complete' || script.readyState == 'loaded') {
        script.onreadystatechange = null;
        callback && callback();
      }
    };
  } else {
    script.onload = function () {
      callback && callback();
    };
  }
}