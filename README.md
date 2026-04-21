# 🕸️ API Scrapper (API-based Web Scraper)

An completely local web scraper that automatically downloads web pages, slices them by HTML heading anchors, converts the content into geometric vectors using local Large Language Models, and stores them in a highly-available indexing database.

Designed natively in **Go** for high-concurrency, backed by **PostgreSQL/pgvector**, and powered locally by **Ollama**.

---

### Features
* **Zero Cost Local AI**: Bypasses OpenAI quotas completely by leveraging Ollama (`nomic-embed-text`) locally.
* **ACID Semantic Database**: Uses a raw Postgres instance loaded with `pgvector` performing zero-latency HNSW Cosine Similarity searches.
* **Beautiful Python UI**: Exposes an interactive Streamlit dashboard allowing users to trigger Go engines and visualize search results seamlessly.

---

### Video Demo Link

https://drive.google.com/file/d/1T_HNJJaIKTg5HkxJVPuD_kxtCSbMHzot/view?usp=sharing

---

###  API Reference
If you prefer not to use the dashboard, you can trigger entirely via `curl`:

**Trigger Background Crawl:**
```bash
curl -X POST http://localhost:8080/start-crawl \
     -H "Content-Type: application/json" \
     -d '{"url":"https://example.com","max_depth":1}'
```

**Execute Semantic Search:**
```bash
curl "http://localhost:8080/search?q=enter%20search%20intent"
```
