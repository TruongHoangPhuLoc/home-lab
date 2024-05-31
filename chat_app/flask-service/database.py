import mysql.connector
import os
mysql_host = os.getenv("MYSQL_HOST")
print(mysql_host)

def get_connection():
    return mysql.connector.connect(
        host= mysql_host,
        user="root",
        password="root",
        database="chat_app"
    )

def authenticate_user(username, hashed_password):
    connection = get_connection()
    cursor = connection.cursor()
    query = "SELECT user_id, username FROM users WHERE username = %s AND password_hash = %s"
    cursor.execute(query, (username, hashed_password))
    user = cursor.fetchone()
    cursor.close()
    connection.close()
    return user

# Add more functions for other database operations as needed

# Getting user
def get_user(username):
    connection = get_connection()
    cursor = connection.cursor()
    query = "SELECT * FROM users WHERE username = %s"
    cursor.execute(query, [username])
    user = cursor.fetchone()
    cursor.close()
    connection.close()
    return user

# Creating user
def create_user(username,hashed_password):
    connection = get_connection()
    cursor = connection.cursor()
    query = "INSERT INTO users (username, password_hash) VALUES (%s, %s)"
    cursor.execute(query, (username, hashed_password))
    connection.commit()  # Commit the transaction
    cursor.close()
    connection.close()

# Getting friend list of user
# TODO, tried to change argument from user_id to user_name
def get_friend_list(user_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = """
        SELECT username
        FROM (
            SELECT username, user_id
            FROM users u
            JOIN friends f ON (u.user_id = f.sender_id OR u.user_id = f.receiver_id)
            WHERE (f.sender_id = %s OR f.receiver_id = %s) AND f.status = 'accepted'
        ) AS subquery
        WHERE user_id != %s;
    """
    cursor.execute(query, (user_id, user_id, user_id))
    friend_list = [friend[0] for friend in cursor.fetchall()]
    cursor.close()
    return friend_list

# Creating friend relationship

def create_friend(user_id, friend_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = "INSERT INTO friends (user_id, friend_id) VALUES (%s, %s)"
    cursor.execute(query, (user_id, friend_id))
    connection.commit()  # Commit the transaction


    #TODO
    # Need to change friend adding mechanism
    # At this time, for simplicity, we create 2 records for each friend adding
    # For example, when user_1 adds friend user_2, we create 2 records for tables friends, 
    # one for user_id = user_1 and user_id = user_2
    # In the future, need to add sent request and approved request functionality
    query = "INSERT INTO friends (user_id, friend_id) VALUES (%s, %s)"
    cursor.execute(query, (friend_id, user_id))
    connection.commit()  # Commit the transaction
    cursor.close()
    connection.close()

def insert_sent_requests(sender_id, receiver_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = "INSERT INTO sent_requests (sender_id, receiver_id) VALUES (%s, %s)"
    cursor.execute(query, (sender_id, receiver_id))
    connection.commit()  # Commit the transaction
    cursor.close()
    connection.close()

def insert_received_requests(sender_id, receiver_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = "INSERT INTO received_requests (sender_id, receiver_id) VALUES (%s, %s)"
    cursor.execute(query, (sender_id, receiver_id))
    connection.commit()  # Commit the transaction
    cursor.close()
    connection.close()

def get_sent_requests(user_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = """
        SELECT u.username, sender_id, receiver_id
        FROM users u
        JOIN sent_requests s ON u.user_id = s.receiver_id
        WHERE s.sender_id = %s and s.hidden = 'false'
    """
    cursor.execute(query, (user_id,))
    sent_list = cursor.fetchall()
    cursor.close()
    return sent_list

#TODO need to correct logic here
def get_received_requests(user_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = """
        SELECT u.username, sender_id, receiver_id
        FROM users u
        JOIN sent_requests s ON u.user_id = s.sender_id
        WHERE s.receiver_id = %s and s.hidden = 'false'
    """
    cursor.execute(query, (user_id,))
    received_list = cursor.fetchall()
    cursor.close()
    return received_list

def insert_friends(sender_id, receiver_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = "INSERT INTO friends (sender_id, receiver_id) VALUES (%s, %s)"
    cursor.execute(query, (sender_id, receiver_id))
    connection.commit()  # Commit the transaction
    cursor.close()
    connection.close()

def get_sent_requests_id(sender_id, receiver_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = "SELECT id FROM sent_requests where sender_id = %s AND receiver_id = %s"
    cursor.execute(query, (sender_id, receiver_id))
    sent_list = cursor.fetchall()
    return sent_list

def accept_request(sender_id, receiver_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = """
        UPDATE friends
        set status = 'accepted'
        where sender_id = %s and receiver_id = %s
    """
    cursor.execute(query,(sender_id, receiver_id))
    connection.commit()
    cursor.close()
    connection.close()
def hide_sent_request(sender_id, receiver_id):
    connection = get_connection()
    cursor = connection.cursor()
    query = """
        UPDATE sent_requests
        set hidden = 'true'
        where sender_id = %s and receiver_id = %s
    """
    cursor.execute(query,(sender_id, receiver_id))
    connection.commit()
    cursor.close()
    connection.close()