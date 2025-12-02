axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

axios.interceptors.request.use(
    (config) => {
        if (config.data === undefined) {
            return config;
        }

        // Skip stringification when explicit JSON is requested
        const contentType =
            (config.headers && (config.headers['Content-Type'] || config.headers['content-type'])) ||
            axios.defaults.headers.post['Content-Type'];
        const isJson = typeof contentType === 'string' && contentType.includes('application/json');

        if (config.data instanceof FormData || isJson) {
            return config;
        }

        config.data = Qs.stringify(config.data, {
            arrayFormat: 'repeat',
        });
        return config;
    },
    (error) => Promise.reject(error),
);

axios.interceptors.response.use(
    (response) => response,
    (error) => {
        if (error.response) {
            const statusCode = error.response.status;
            // Check the status code
            if (statusCode === 401) { // Unauthorized
                return window.location.reload();
            }
        }
        return Promise.reject(error);
    }
);
