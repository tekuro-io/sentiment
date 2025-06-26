from flask import Flask, render_template
import openai
import requests
import os

app = Flask(__name__)

# === CONFIG ===
serpapi_api_key = os.getenv("SEARCH_KEY")
client = openai.OpenAI(api_key=os.getenv("OPENAI_KEY"))

def ask_gpt(ticker, web_results):
    print(f"🤖 Sending web results to OpenAI for ticker: {ticker}")
    try:
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

        response = client.chat.completions.create(
            model="gpt-4o",
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt}
            ],
            temperature=0.3
        )

        gpt_reply = response.choices[0].message.content.strip()
        print(f"✅ GPT response for {ticker}:\n{gpt_reply}\n")
        return gpt_reply

    except Exception as e:
        print(f"❌ OpenAI API Error for {ticker}: {e}")
        return f"Error from GPT API: {str(e)}"


def get_web_results(ticker):
    print(f"🔎 Searching news for ticker: {ticker}")
    try:
        search_url = "https://serpapi.com/search.json"
        params = {
            "engine": "google",
            "q": f"{ticker} stock news",
            "api_key": serpapi_api_key,
            "num": 5
        }
        response = requests.get(search_url, params=params)
        response.raise_for_status()
        data = response.json()

        snippets = []
        for result in data.get("organic_results", []):
            title = result.get("title", "")
            snippet = result.get("snippet", "")
            link = result.get("link", "")
            snippets.append(f"- {title}\n{snippet}\n({link})")

        combined_results = "\n".join(snippets[:5]) if snippets else "No recent news found."
        print(f"✅ Web search results for {ticker}:\n{combined_results}\n")
        return combined_results

    except Exception as e:
        print(f"❌ Error fetching web results for {ticker}: {e}")
        return f"Error fetching web results for {ticker}: {str(e)}"


def handle_null_var():
    print("⚠️ Null ticker received.")
    return "No ticker provided."

def handle_ticker(ticker):
    print(f"🛠️ Handling ticker: {ticker}")
    web_results = get_web_results(ticker)

    if web_results.startswith("Error fetching"):
        return web_results  # Skip GPT if web fetch failed

    gpt_output = ask_gpt(ticker, web_results)
    return gpt_output if gpt_output else "No analysis returned."

@app.route('/get/', defaults={'var': None})
@app.route('/get/<var>')
def get_var(var):
    if var is None:
        data = handle_null_var()
        return render_template('null_page.html', data=data)
    else:
        ai_text = handle_ticker(var)
        return render_template('sentiment.html', ai_text=ai_text, ticker=var.upper())

if __name__ == '__main__':
    app.run(host='0.0.0.0', debug=True)
