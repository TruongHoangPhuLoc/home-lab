from flask import Flask, render_template, request, session, redirect, url_for, jsonify
import os
import database
import hashlib
app = Flask(__name__)
app.secret_key = 'your_secret_key'  # Change this to a random secret key
ws_host = os.getenv("WS_HOST")

@app.route('/', methods=['GET', 'POST'])
def login():
    if request.method == 'POST':
        username = request.form['username']
        password = request.form['password']  
        # Hash the provided password
        hashed_password = hashlib.sha256(password.encode()).hexdigest() 
        # Query the database for the user
        user = database.authenticate_user(username,hashed_password)
        if user:
            # Set session for the user
            session['user_id'] = user[0]
            session['username'] = user[1]
            return redirect(url_for('home'))
        else:
            return 'Invalid username or password'
    
    return render_template('login.html')

@app.route('/register', methods=['GET', 'POST'])
def register():
    if request.method == 'POST':
        username = request.form['username']
        password = request.form['password']
        
        # Check if username already exists in the database
        existing_user = database.get_user(username)
        if existing_user:
            return 'Username already exists'
        
        # Insert new user into the database
        hashed_password = hashlib.sha256(password.encode()).hexdigest()
        database.create_user(username, hashed_password)
        return redirect(url_for('login'))
    
    return render_template('register.html')

@app.route('/home')
def home():
    if 'username' in session and 'user_id' in session:
        username = session['username']
        userid = session['user_id']
        friend_list = database.get_friend_list(userid)
        sent_requests = database.get_sent_requests(userid)
        received_requests = database.get_received_requests(userid)
        print(received_requests)
        print(userid)
        return render_template('home.html', userid=userid,username=username, friend_list=friend_list, sent_requests=sent_requests, received_requests=received_requests, ws_host=ws_host)
    return redirect(url_for('login'))

@app.route('/logout',  methods=['GET', 'POST'])
def logout():
    if request.method == 'POST':
        session.clear()
        return redirect(url_for('login'))

# Friend Adding
@app.route('/send_request', methods=['POST'])
def send_request():
    if 'username' in session:
        sender_id = session['user_id']
        data = request.json  # Assuming the data sent is in JSON format
        receiver_username = data.get('receiver_username')  # Assuming the key in the JSON is 'receiver_username'
        receiver_id = database.get_user(receiver_username)[0]
        
        if sender_id and receiver_id:
            database.insert_sent_requests(sender_id, receiver_id)
            database.insert_friends(sender_id, receiver_id)
            return jsonify({'message': 'Friend request sent successfully'})
        else:
            return jsonify({'error': 'Sender or receiver not found'})
    else:
        return jsonify({'error': 'User not logged in'})

# Accepting Request
@app.route('/accept_request', methods=['POST'])
def request_accepting():
    if request.method == 'POST':
        data = request.json
        sender_id = data.get('sender_id')
        receiver_id = data.get('receiver_id')
        if sender_id and receiver_id:
            database.accept_request(sender_id, receiver_id)
            # Hiding that request
            database.hide_sent_request(sender_id, receiver_id)
            return jsonify({'message': 'Request has been accepted'})
        else:
            return jsonify({'message': 'Friend_id not found'})
    else:
        return jsonify({'message': 'Method is not allowed'})


#Test websocket
@app.route('/test')

@app.route('/requests')
def get_request_page():
    userid = session['user_id']
    sent_requests = database.get_sent_requests(userid)
    received_requests = database.get_received_requests(userid)
    return render_template('requests.html',sent_requests=sent_requests, received_requests=received_requests,ws_host=ws_host,userid=userid)
def test():
    return render_template('test-socket.html')
if __name__ == '__main__':
    app.run(debug=True)
