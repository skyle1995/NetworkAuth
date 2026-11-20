import { h, defineComponent, type Component, type PropType } from "vue";
import { Icon as IconifyIcon } from "@iconify/vue/dist/offline";
import type { IconifyIcon as IconData } from "@iconify/vue";

function isIconData(icon: IconData | Component): icon is IconData {
  return typeof icon === "object" && "body" in icon && typeof icon.body === "string";
}

// Iconify Icon在Vue里本地使用（用于内网环境）
export default defineComponent({
  name: "IconifyIconOffline",
  components: { IconifyIcon },
  props: {
    icon: {
      type: [String, Object, Function] as PropType<string | IconData | Component>,
      default: null
    }
  },
  render() {
    const attrs = this.$attrs;
    if (!this.icon) return null;
    const props = { "aria-hidden": false, ...attrs, style: [attrs.style, { outline: "none" }] };
    if (typeof this.icon === "string" || isIconData(this.icon)) {
      return h(IconifyIcon, { ...props, icon: this.icon });
    }
    return h(this.icon, props);
  }
});
