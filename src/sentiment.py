from flask import Flask, render_template, request

app = Flask(__name__)

def handle_null_var():
    # Your logic for handling null var
    data = "No variable was provided."
  
    return data

def handle_ticker(var):
 
    data = f"The variable provided is: {var}"

    return data

@app.route('/get/', defaults={'var': None})
@app.route('/get/<var>')
def get_var(var):
    if var is None:
        data = handle_null_var()
        return render_template('null_page.html', data=data)
    else:
        data = handle_ticker(var)
        return render_template('sentiment.html', data=data)

if __name__ == '__main__':
    app.run(debug=True)
