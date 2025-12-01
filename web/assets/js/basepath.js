(() => {
  const script = document.currentScript || document.getElementById('basepath-script');
  const basePath = (script && script.dataset && script.dataset.basePath) || '/';
  window.basePath = basePath;
  if (window.axios) {
    axios.defaults.baseURL = basePath;
  }
})();
