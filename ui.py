import streamlit as st
import requests
import time

API_URL = "http://localhost:8080"

st.set_page_config(
    page_title="Scraper & Vector Search UI",
    page_icon="🕸️",
    layout="wide",
)

st.title("🕸️ API Scraper & Vector Knowledge Graph")
st.markdown("A completely local, AI-powered web scraper using **Go** + **PostgreSQL/pgvector** + **Ollama**.")

with st.sidebar:
    st.header("Start a Crawl")
    st.markdown("Give the engine an initial URL to start scraping.")
    
    target_url = st.text_input("Target URL", placeholder="https://example.com", help="The starting point for the scraper. It will crawl this page and follow links up to the specified depth.")
    max_depth = st.number_input("Max Depth", min_value=1, max_value=5, value=1)
    
    if st.button("Trigger Scrape", type="primary"):
        with st.spinner("Dispatching to Go engine..."):
            try:
                res = requests.post(f"{API_URL}/start-crawl", json={"url": target_url, "max_depth": max_depth})
                if res.status_code == 200:
                    st.success(f"Dispatched background scrape for: {target_url} Check your Go terminal for live logs!")
                else:
                    st.error(f"Error ({res.status_code}): {res.text}")
            except requests.exceptions.ConnectionError:
                st.error("Could not connect to the Go API! Is `go run main.go` running on port 8080?")


st.header("Semantic Search")
st.markdown("Query the vectorized database to find specific information across all downloaded pages and anchor nodes.")

query = st.text_input("Search Intent", placeholder="--intent--", help="E.g. 'What does the homepage say about their product offerings?'")

if st.button("⚡ Search Database"):
    if query.strip() == "":
        st.warning("Please specify a query first.")
    else:
        with st.spinner("Generating embeddings via Ollama & querying pgvector..."):
            try:
                res = requests.get(f"{API_URL}/search", params={"q": query})
                if res.status_code == 200:
                    data = res.json()
                    results = data.get("results", [])
                    
                    if not results:
                        st.info("No matching chunks found in the database. Have you crawled any pages successfully yet?")
                    else:
                        st.success(f"Found {len(results)} highly-relevant database chunks.")
                        
                        for idx, item in enumerate(results):
                            with st.expander(f"{item.get('title', 'Unknown Page')} (Score: {item.get('score', 0):.4f})", expanded=(idx==0)):
                                st.markdown(f"**Source URL:** [{item['url']}]({item['url']})")
                                st.markdown("---")
                                st.write(item.get("content", ""))
                else:
                    st.error(f"Error ({res.status_code}): {res.text}")
            except requests.exceptions.ConnectionError:
                st.error("Could not connect to the Go API! Is `go run main.go` running on port 8080?")
