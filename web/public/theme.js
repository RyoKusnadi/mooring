// Runs before first paint (see index.html) so the chosen theme and text size show without a flash.
// The same keys and rules live in src/lib/appearance.ts.
;(function () {
  var theme = 'system'
  var size = 'default'
  try {
    var t = localStorage.getItem('mooring.theme')
    var s = localStorage.getItem('mooring.fontsize')
    if (t === 'light' || t === 'dark') theme = t
    if (s === 'small' || s === 'large') size = s
  } catch (e) {}
  var dark = theme === 'dark' || (theme !== 'light' && matchMedia('(prefers-color-scheme: dark)').matches)
  var root = document.documentElement
  root.dataset.theme = dark ? 'dark' : 'light'
  root.dataset.size = size
})()
