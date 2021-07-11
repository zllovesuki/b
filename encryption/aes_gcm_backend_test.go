package encryption

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/zllovesuki/b/app"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type testDependencies struct {
	mockBackend *app.MockBackend
	AESGCM      *AESGCM
}

func getFixtures(t *testing.T, keyLength int) (*testDependencies, func()) {
	ctrl := gomock.NewController(t)
	mockBackend := app.NewMockBackend(ctrl)

	key := make([]byte, keyLength)
	_, err := io.ReadFull(rand.Reader, key)
	require.NoError(t, err)

	e, err := NewAESGCMBackend(mockBackend, key)
	require.NoError(t, err)

	return &testDependencies{
			mockBackend: mockBackend,
			AESGCM:      e,
		}, func() {
			ctrl.Finish()
		}
}

func TestInvalidKeySize(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockBackend := app.NewMockBackend(ctrl)
	defer ctrl.Finish()

	length := []int{}
	for i := 1; i < 48; i++ {
		if i == 16 || i == 24 || i == 32 {
			continue
		}
		length = append(length, i)
	}
	for _, l := range length {
		key := make([]byte, l)
		r, err := rand.Read(key)
		require.NoError(t, err)
		require.Equal(t, l, r)

		e, err := NewAESGCMBackend(mockBackend, key)
		require.Error(t, err)
		require.Nil(t, e)
	}
}

func TestAESGCM(t *testing.T) {
	keyLength := []int{
		16,
		24,
		32,
	}
	for _, length := range keyLength {
		t.Run(fmt.Sprintf("key size: %d", length), func(t *testing.T) {
			t.Run("happy path", func(t *testing.T) {
				f, cleanup := getFixtures(t, length)
				defer cleanup()

				id := "id"

				clearText := make([]byte, 1024)
				_, err := io.ReadFull(rand.Reader, clearText)
				require.NoError(t, err)

				cipherText := []byte{}

				f.mockBackend.EXPECT().
					SaveTTL(gomock.Any(), id, gomock.Any(), time.Duration(0)).
					DoAndReturn(func(c context.Context, identifier string, data []byte, ttl time.Duration) interface{} {
						cipherText = append(cipherText, data...)
						return nil
					})

				err = f.AESGCM.SaveTTL(context.Background(), id, []byte(clearText), 0)
				require.NoError(t, err)

				f.mockBackend.EXPECT().
					Retrieve(gomock.Any(), id).
					Return(cipherText, nil)

				plain, err := f.AESGCM.Retrieve(context.Background(), id)
				require.NoError(t, err)
				require.Equal(t, clearText, plain)
			})

			t.Run("should fail on", func(t *testing.T) {
				where := []struct {
					Description string
					How         func(cipherText []byte)
				}{
					{
						Description: "manipulated cipher text",
						How: func(cipherText []byte) {
							rand.Reader.Read(cipherText[12:18])
						},
					},
					{
						Description: "manipulated nonce",
						How: func(cipherText []byte) {
							rand.Reader.Read(cipherText[0:6])
						},
					},
					{
						Description: "manipulated tag",
						How: func(cipherText []byte) {
							rand.Reader.Read(cipherText[len(cipherText)-8:])
						},
					},
				}

				for _, w := range where {
					t.Run(w.Description, func(t *testing.T) {
						f, cleanup := getFixtures(t, length)
						defer cleanup()

						id := "id"
						text := "hello world!"
						cipherText := []byte{}

						f.mockBackend.EXPECT().
							SaveTTL(gomock.Any(), id, gomock.Any(), time.Duration(0)).
							DoAndReturn(func(c context.Context, identifier string, data []byte, ttl time.Duration) interface{} {
								cipherText = append(cipherText, data...)
								return nil
							})

						err := f.AESGCM.SaveTTL(context.Background(), id, []byte(text), 0)
						require.NoError(t, err)

						w.How(cipherText)

						f.mockBackend.EXPECT().
							Retrieve(gomock.Any(), id).
							Return(cipherText, nil)

						_, err = f.AESGCM.Retrieve(context.Background(), id)
						require.Error(t, err)
					})
				}
			})
		})
	}
}
