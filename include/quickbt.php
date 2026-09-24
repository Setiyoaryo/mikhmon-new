<?php
if (is_file(dirname(__FILE__) . '/../data/quickbt.php')) {
  include(dirname(__FILE__) . '/../data/quickbt.php');
} else {
  $qrbt = "disable";
}
?>