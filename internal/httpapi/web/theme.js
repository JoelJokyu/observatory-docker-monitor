try {
  document.documentElement.dataset.theme =
    localStorage.getItem('observatory-theme') === 'light' ? 'light' : 'dark';
} catch {
  document.documentElement.dataset.theme = 'dark';
}
