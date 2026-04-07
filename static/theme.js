(function() {
    var storageKey = 'theme-preference';
    var media = window.matchMedia('(prefers-color-scheme: dark)');

    function getPreference() {
        try {
            var value = window.localStorage.getItem(storageKey);
            if (value === 'light' || value === 'dark' || value === 'system') {
                return value;
            }
        } catch (e) {
        }
        return 'system';
    }

    function resolveTheme(preference) {
        if (preference === 'light' || preference === 'dark') {
            return preference;
        }
        return media.matches ? 'dark' : 'light';
    }

    function applyTheme(preference) {
        var resolved = resolveTheme(preference);
        document.documentElement.setAttribute('data-theme-preference', preference);
        if (preference === 'system') {
            document.documentElement.removeAttribute('data-theme');
        } else {
            document.documentElement.setAttribute('data-theme', preference);
        }
        document.documentElement.style.colorScheme = resolved;
    }

    function persistTheme(preference) {
        try {
            window.localStorage.setItem(storageKey, preference);
        } catch (e) {
        }
    }

    function nextPreference(preference) {
        if (preference === 'system') return 'light';
        if (preference === 'light') return 'dark';
        return 'system';
    }

    function themeIcon(preference) {
        if (preference === 'light') {
            return '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2.5"/><path d="M12 19.5V22"/><path d="M4.93 4.93l1.77 1.77"/><path d="M17.3 17.3l1.77 1.77"/><path d="M2 12h2.5"/><path d="M19.5 12H22"/><path d="M4.93 19.07l1.77-1.77"/><path d="M17.3 6.7l1.77-1.77"/></svg>';
        }
        if (preference === 'dark') {
            return '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>';
        }
        return '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8"/><path d="M12 16v4"/></svg>';
    }

    function themeLabel(preference) {
        if (preference === 'light') return 'Theme: Light';
        if (preference === 'dark') return 'Theme: Dark';
        return 'Theme: System';
    }

    function updateButtons() {
        var preference = getPreference();
        document.querySelectorAll('[data-theme-toggle]').forEach(function(button) {
            button.innerHTML = themeIcon(preference);
            var label = themeLabel(preference);
            button.title = label;
            button.setAttribute('aria-label', label);
            button.dataset.themeState = preference;
        });
    }

    applyTheme(getPreference());

    document.addEventListener('DOMContentLoaded', function() {
        updateButtons();
    });

    document.addEventListener('click', function(e) {
        var button = e.target.closest('[data-theme-toggle]');
        if (!button) return;
        var preference = nextPreference(getPreference());
        persistTheme(preference);
        applyTheme(preference);
        updateButtons();
    });

    if (media.addEventListener) {
        media.addEventListener('change', function() {
            if (getPreference() === 'system') {
                applyTheme('system');
                updateButtons();
            }
        });
    }
})();
