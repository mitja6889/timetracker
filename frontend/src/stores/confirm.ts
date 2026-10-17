import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ConfirmOptions = {
  title?: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  /** danger = rdeč gumb za destruktivne akcije */
  variant?: 'default' | 'danger'
}

export const useConfirmStore = defineStore('confirm', () => {
  const open = ref(false)
  const options = ref<ConfirmOptions | null>(null)
  let resolver: ((value: boolean) => void) | null = null

  function confirm(opts: ConfirmOptions | string): Promise<boolean> {
    if (open.value) {
      return Promise.resolve(false)
    }
    return new Promise((resolve) => {
      options.value = typeof opts === 'string' ? { message: opts } : opts
      open.value = true
      resolver = resolve
    })
  }

  function answer(value: boolean) {
    open.value = false
    const r = resolver
    resolver = null
    options.value = null
    r?.(value)
  }

  return { open, options, confirm, answer }
})

export function useConfirm() {
  return useConfirmStore().confirm
}
