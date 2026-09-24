<?php
/*
 * Contoh Custom Login Page untuk Panel Mikhmon
 * File ini dipanggil saat user membuka https://<customer>.nocify.id/admin.php?id=login
 */
session_start();
?>
<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Login - Panel Hotspot</title>
  <link rel="stylesheet" href="./css/mikhmon-ui.dark.min.css">
  <style>
    body { background: #1a1e24; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; font-family: sans-serif; }
    .box-login { background: #22272e; padding: 30px; border-radius: 8px; width: 100%; max-width: 360px; box-shadow: 0 4px 15px rgba(0,0,0,0.5); }
    .btn-submit { width: 100%; padding: 10px; background: #007bff; color: white; border: none; border-radius: 4px; font-weight: bold; cursor: pointer; margin-top: 15px; }
    .btn-submit:hover { background: #0056b3; }
    .input-field { width: 100%; padding: 10px; margin-top: 10px; border-radius: 4px; border: 1px solid #444; background: #2d333b; color: white; box-sizing: border-box; }
    .error { background: #dc3545; color: white; padding: 8px; border-radius: 4px; margin-bottom: 10px; text-align: center; }
  </style>
</head>
<body>
  <div class="box-login">
    <h2 style="color: white; text-align: center; margin-top: 0;">Portal Management</h2>
    <?php if (!empty($error)) { ?>
      <div class="error"><?= $error; ?></div>
    <?php } ?>
    <form method="post" action="">
      <input class="input-field" type="text" name="user" placeholder="Username" required autofocus>
      <input class="input-field" type="password" name="pass" placeholder="Password" required>
      <button class="btn-submit" type="submit" name="login" value="1">Masuk</button>
    </form>
  </div>
</body>
</html>
