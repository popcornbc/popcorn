for i in {0..2}
do
    echo $i   
    ./popCli -id=$i &
done
