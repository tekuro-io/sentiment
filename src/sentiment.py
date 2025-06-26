from flask import Flask, render_template, request
import openai
import requests
import os

app = Flask(__name__)

# === CONFIG ===
openai.api_key = os.getenv("OPENAI_KEY")
serpapi_api_key = os.getenv("SEARCH_KEY")

def get_web_results(ticker):
    """Use SerpAPI to get Google search results for the ticker"""
    search_url = "https://serpapi.com/search.json"
    params = {
        "engine": "google",
        "q": f"{ticker} stock news",
        "api_key": serpapi_api_key,
        "num": 5
    }
    response = requests.get(search_url, params=params)
    data = response.json()

    snippets = []
    for result in data.get("organic_results", []):
        title = result.get("title", "")
        snippet = result.get("snippet", "")
        link = result.get("link", "")
        snippets.append(f"- {title}\n{snippet}\n({link})")

    return "\n".join(snippets[:5]) if snippets else "No recent news found."

def ask_gpt(ticker, web_results):
    """Send search results and ticker info to OpenAI for analysis"""
    system_prompt = (
        "You are a professional stock market news analyst. "
        "Given web search results about a stock, summarize:\n"
        "- What the company does (1-2 lines)\n"
        "- Today's main catalyst or news moving the stock\n"
        "- Sentiment (Bullish, Bearish, Neutral) and why\n"
        "- Possible intraday price action or volatility range estimate"
    )

    user_prompt = (
        f"Ticker: {ticker}\n\n"
        f"Web Search Results:\n{web_results}\n\n"
        "Give your analysis:"
    )

    response = openai.ChatCompletion.create(
        model="gpt-4o",
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt}
        ],
        temperature=0.3
    )

    return response.choices[0].message.content.strip()

def handle_null_var():
    """For null page"""
    return "No ticker provided."

def handle_ticker(ticker):
    """Main logic for sentiment check"""
    try:
        web_results = get_web_results(ticker)
        gpt_output = ask_gpt(ticker, web_results)
        return gpt_output
    except Exception as e:
        return f"Error fetching sentiment for {ticker}: {str(e)}"

@app.route('/get/', defaults={'var': None})
@app.route('/get/<var>')
def get_var(var):
    if var is None:
        data = handle_null_var()
        return render_template('null_page.html', data=data)
    else:
        ai_text = handle_ticker(var)
        return render_template('sentiment.html', ai_text=ai_text)

if __name__ == '__main__':
    app.run(host='0.0.0.0', debug=True)
