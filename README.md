# GoBlog-Letterboxd

A [PESOS](https://indieweb.org/PESOS) plugin that uses RSS and Micropub for crossposting Letterboxd diary entries to a [GoBlog](https://github.com/jlelse/GoBlog) instance.

## Config
```yaml
- path: ./plugins/letterboxd
    import: letterboxd
    config:
      username: "user" # Letterboxd Username
      blogURL: "http://goblog.url" # GoBlog instance URL
      section: "section" # GoBlog's Watches Section
      token: "MICROPUB-TOKEN" # GoBlog's Micropub Token
```

**Demo:** 📺 [Watches](https://kandr3s.co/watches)

---

### TO-DO

- [x] Add Microformats in Watches (Fetched from Letterboxd)
- [x] Fetch data directly from the Letterboxd feed
- [x] Implement “Rewatched”
- [x] Set up variables in GoBlog's config
- [ ] Automatically fetch movie backdrops 