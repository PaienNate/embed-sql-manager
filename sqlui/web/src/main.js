import 'url-polyfill';
import ResizeObserverPolyfill from 'resize-observer-polyfill';

if (typeof window !== 'undefined' && !window.ResizeObserver) {
  window.ResizeObserver = ResizeObserverPolyfill;
}

import Vue from 'vue';
import ElementUI from 'element-ui';
import './assets/fontawesome-subset.css';
import 'vxe-table/lib/style.css';
import './styles.css';
import App from './App';

Vue.use(ElementUI, { size: 'small' });
Vue.config.productionTip = false;

new Vue({
  el: '#app',
  // Runtime-only Vue build: mount with a render function (no template compiler).
  render: (h) => h(App),
});
