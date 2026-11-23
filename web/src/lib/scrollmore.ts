// Svelte action: sets data-more on a scroll box while there is more to scroll down to.
// Used for the fade at the bottom of the file tree, which must not lie over the last row.
export function scrollMore(node: HTMLElement) {
  const update = () => {
    const more = node.scrollHeight - node.scrollTop - node.clientHeight > 1
    if (more) node.dataset.more = ''
    else delete node.dataset.more
  }
  const resize = new ResizeObserver(update)
  const content = new MutationObserver(update)
  resize.observe(node)
  content.observe(node, { childList: true, subtree: true })
  node.addEventListener('scroll', update, { passive: true })
  update()
  return {
    destroy() {
      resize.disconnect()
      content.disconnect()
      node.removeEventListener('scroll', update)
    },
  }
}
