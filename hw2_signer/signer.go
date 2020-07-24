package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

func SingleHash(in, out chan interface{}) {
	waitGroup := &sync.WaitGroup{}
	mutex := &sync.Mutex{}
	for inItem := range in {
		waitGroup.Add(1)
		go func(inItem interface{}, out chan interface{}, waitGroup *sync.WaitGroup) {
			defer waitGroup.Done()
			data := strconv.Itoa(inItem.(int))
			md5Crc32Chan := make(chan string)
			crc32Chan := make(chan string)
			go func(out chan string, data string) {
				mutex.Lock()
				md5Hash := DataSignerMd5(data)
				mutex.Unlock()
				out <- DataSignerCrc32(md5Hash)
			}(md5Crc32Chan, data)
			go func(out chan string, data string) {
				out <- DataSignerCrc32(data)
			}(crc32Chan, data)

			md5Crc32 := <-md5Crc32Chan
			crc32 := <-crc32Chan
			out <- crc32 + "~" + md5Crc32
		}(inItem, out, waitGroup)

	}
	waitGroup.Wait()
}

func MultiHash(in, out chan interface{}) {
	waitGroupTop := &sync.WaitGroup{}
	for input := range in {
		waitGroupTop.Add(1)
		go func(in interface{}) {
			defer waitGroupTop.Done()
			data, _ := in.(string)
			waitGroup := &sync.WaitGroup{}
			mutex := &sync.Mutex{}

			hashData := make(map[int]string, 6)
			for index := 0; index < 6; index++ {
				waitGroup.Add(1)
				go func(hashData map[int]string, index int, data string) {
					defer waitGroup.Done()
					hash := DataSignerCrc32(strconv.Itoa(index) + data)
					mutex.Lock()
					hashData[index] = hash
					mutex.Unlock()
				}(hashData, index, data)
			}
			waitGroup.Wait()

			keys := make([]int, 0, len(hashData))
			for k, _ := range hashData {
				keys = append(keys, k)
			}
			sort.Ints(keys)

			var result string
			for k := range keys {
				result += hashData[k]
			}

			out <- result
		}(input)
	}
	waitGroupTop.Wait()
}

func CombineResults(in, out chan interface{}) {
	var hashes []string
	for input := range in {
		data, _ := input.(string)
		hashes = append(hashes, data)
	}
	sort.Strings(hashes)

	result := strings.Join(hashes, "_")
	out <- result
}

func ExecutePipeline(jobs ...job) {
	in := make(chan interface{})
	out := make(chan interface{})

	waitGroup := &sync.WaitGroup{}
	for _, jobItem := range jobs {
		waitGroup.Add(1)
		go func(waiter *sync.WaitGroup, jobItem job, in, out chan interface{}) {
			defer waiter.Done()
			defer close(out)
			jobItem(in, out)
		}(waitGroup, jobItem, in, out)
		in = out
		out = make(chan interface{})
	}

	waitGroup.Wait()
}
