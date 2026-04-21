-- Create the database
CREATE DATABASE IF NOT EXISTS chat_app;

-- Use the newly created database
USE chat_app;

-- Create users table
CREATE TABLE users (
    user_id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(128) NOT NULL,
    email VARCHAR(100),
    full_name VARCHAR(100)
);

-- Create chat_rooms table
CREATE TABLE chat_rooms (
    room_id INT AUTO_INCREMENT PRIMARY KEY,
    user1_id INT,
    user2_id INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user1_id) REFERENCES users(user_id),
    FOREIGN KEY (user2_id) REFERENCES users(user_id)
);

-- Create messages table
CREATE TABLE messages (
    message_id INT AUTO_INCREMENT PRIMARY KEY,
    room_id INT,
    sender_id INT,
    receiver_id INT,
    content TEXT,
    sent_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (room_id) REFERENCES chat_rooms(room_id),
    FOREIGN KEY (sender_id) REFERENCES users(user_id),
    FOREIGN KEY (receiver_id) REFERENCES users(user_id)
);
CREATE TABLE sent_requests (
    sender_id INT NOT NULL,
    receiver_id INT NOT NULL,
    hidden ENUM('true', 'false') DEFAULT 'false' NOT NULL,
    PRIMARY KEY (sender_id, receiver_id),
    FOREIGN KEY (sender_id) REFERENCES users(user_id),
    FOREIGN KEY (receiver_id) REFERENCES users(user_id)
);
CREATE TABLE friends (
    sender_id INT,
    receiver_id INT,
    PRIMARY KEY(sender_id, receiver_id),
    status ENUM('pending', 'accepted') DEFAULT 'pending' NOT NULL,
    FOREIGN KEY (sender_id) REFERENCES sent_requests(sender_id),
    FOREIGN KEY (receiver_id) REFERENCES sent_requests(receiver_id)
);




